// Command eval produces the thesis evaluation data: a paced load run through the dispute lifecycle with one tenant
// key, then the latency and outcome numbers read back from Prometheus for exactly that window. Output is JSON so
// the numbers in the text are copied, never typed.
//
//	eval --api http://localhost:8090 --key tk_dev_tenant_a --prometheus http://localhost:9090 --rps 5 --seconds 120 --out load.json
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Seeded OTP Bank transactions (cmd/seed); each run opens fresh disputes against them.
var transactions = []string{
	"00000000-0000-8000-8000-000000000101",
	"00000000-0000-8000-8000-000000000102",
	"00000000-0000-8000-8000-000000000103",
	"00000000-0000-8000-8000-000000000201",
	"00000000-0000-8000-8000-000000000202",
}

// The probe account has no email or postal address, so a run against it exercises everything but the mail relay
// and fills no inbox.
var silentTransactions = []string{
	"00000000-0000-8000-8000-000000000901",
	"00000000-0000-8000-8000-000000000902",
}

type result struct {
	Started       time.Time         `json:"started"`
	Ended         time.Time         `json:"ended"`
	TargetRPS     float64           `json:"targetRps"`
	AchievedRPS   float64           `json:"achievedRps"`
	Requests      int64             `json:"requests"`
	Statuses      map[string]int64  `json:"statuses"`
	Replays       int64             `json:"idempotentReplays"`
	Disputes      int64             `json:"disputesOpened"`
	Transitions   int64             `json:"transitionsApplied"`
	Latency       map[string]any    `json:"latencyByRoute"`
	Problems      map[string]any    `json:"problemsByCode"`
	Notes         []string          `json:"notes,omitempty"`
	ClientLatency map[string]string `json:"clientLatencyOverall"`
	Workbench     *workbenchResult  `json:"workbench,omitempty"`
	Runtime       map[string]any    `json:"runtime,omitempty"`
}

// workbenchResult is the frontend-server half of a run: page loads through the session, and the workbench's own
// metrics for the window.
type workbenchResult struct {
	TargetPagesPerSecond float64           `json:"targetPagesPerSecond"`
	Pages                int64             `json:"pages"`
	Statuses             map[string]int64  `json:"statuses"`
	ClientLatency        map[string]string `json:"clientLatencyOverall"`
	PageLatency          map[string]any    `json:"pageLatencyByRoute"`
	ServerFnLatency      map[string]any    `json:"serverFnLatencyByFunction"`
}

func main() {
	api := flag.String("api", "http://localhost:8090", "API base URL")
	key := flag.String("key", "tk_dev_tenant_a", "tenant key")
	prom := flag.String("prometheus", "http://localhost:9090", "Prometheus base URL")
	rps := flag.Float64("rps", 5, "lifecycle iterations per second (each is 4 to 5 requests)")
	seconds := flag.Int("seconds", 120, "run length")
	out := flag.String("out", "load.json", "output file")
	silent := flag.Bool("silent", false, "open disputes on the probe account, which has no address, so no mail is sent")
	workbench := flag.String("workbench", "", "workbench base URL; with --session, pages are loaded through it during the run")
	session := flag.String("session", "", "a signed-in de_session cookie value for the workbench (see frontend/scripts/session-cookie.ts)")
	pagesPerSecond := flag.Float64("pages", 2, "workbench page loads per second")
	flag.Parse()
	if *silent {
		transactions = silentTransactions
	}
	res, err := run(context.Background(), runOptions{api: *api, key: *key, prom: *prom, rps: *rps, d: time.Duration(*seconds) * time.Second,
		workbench: *workbench, session: *session, pages: *pagesPerSecond})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
	fmt.Printf("eval: %d requests at %.1f/s, %d disputes, %d transitions, %d replays; wrote %s\n",
		res.Requests, res.AchievedRPS, res.Disputes, res.Transitions, res.Replays, *out)
}

type client struct {
	base, key string
	http      *http.Client
	statuses  sync.Map
	requests  atomic.Int64
	latencies []time.Duration
	mu        sync.Mutex
}

func (c *client) do(ctx context.Context, method, path string, body any, idem string) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequestWithContext(ctx, method, c.base+path, &buf)
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	elapsed := time.Since(start)
	c.requests.Add(1)
	c.mu.Lock()
	c.latencies = append(c.latencies, elapsed)
	c.mu.Unlock()
	if err != nil {
		c.count("error")
		return 0, nil
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body
	c.count(fmt.Sprint(resp.StatusCode))
	var out map[string]any
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func (c *client) count(k string) {
	v, _ := c.statuses.LoadOrStore(k, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
}

// One iteration: open a dispute (with an idempotency key), replay it once in ten, read it, then apply up to two
// allowed events chosen from what the API says is allowed, and deliberately try one refused event.
func iteration(ctx context.Context, c *client, replays, disputes, transitions *atomic.Int64, ids *opened) {
	txn := transactions[rand.IntN(len(transactions))] //nolint:gosec // load mix, not a secret
	idem := uuid.NewString()
	body := map[string]any{"transactionId": txn, "actor": "eval"}
	code, d := c.do(ctx, http.MethodPost, "/disputes", body, idem)
	if code != 201 {
		return
	}
	disputes.Add(1)
	id, _ := d["id"].(string)
	ids.add(id)
	if rand.IntN(10) == 0 { //nolint:gosec // load mix, not a secret
		if code, _ := c.do(ctx, http.MethodPost, "/disputes", body, idem); code == 201 {
			replays.Add(1)
		}
	}
	_, d = c.do(ctx, http.MethodGet, "/disputes/"+id, nil, "")
	for range 2 {
		allowed, _ := d["allowedEvents"].([]any)
		if len(allowed) == 0 {
			break
		}
		ev := allowed[rand.IntN(len(allowed))] //nolint:gosec // load mix, not a secret
		var next int
		next, d = c.do(ctx, http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": ev, "actor": "eval"}, uuid.NewString())
		if next == 200 {
			transitions.Add(1)
		}
	}
	c.do(ctx, http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "WIN_CHARGEBACK", "actor": "eval"}, uuid.NewString())
}

type runOptions struct {
	api, key, prom     string
	rps                float64
	d                  time.Duration
	workbench, session string
	pages              float64
}

// opened is the ids the API loop has created so far; the workbench loop reads pages for them.
type opened struct {
	mu  sync.Mutex
	ids []string
}

func (o *opened) add(id string) { o.mu.Lock(); o.ids = append(o.ids, id); o.mu.Unlock() }
func (o *opened) pick() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.ids) == 0 {
		return ""
	}
	return o.ids[rand.IntN(len(o.ids))] //nolint:gosec // load mix, not a secret
}

// page loads a workbench page the way a browser would: server-rendered, with the session cookie, no token.
func page(ctx context.Context, wb *client, session, path string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, wb.base+path, nil)
	req.AddCookie(&http.Cookie{Name: "de_session", Value: session, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}) // a request cookie; the attributes are inert here and satisfy gosec
	start := time.Now()
	resp, err := wb.http.Do(req)
	wb.requests.Add(1)
	wb.mu.Lock()
	wb.latencies = append(wb.latencies, time.Since(start))
	wb.mu.Unlock()
	if err != nil {
		wb.count("error")
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	wb.count(fmt.Sprint(resp.StatusCode))
}

func percentiles(c *client) map[string]string {
	out := map[string]string{}
	c.mu.Lock()
	defer c.mu.Unlock()
	sort.Slice(c.latencies, func(i, j int) bool { return c.latencies[i] < c.latencies[j] })
	for _, q := range []struct {
		name string
		p    float64
	}{{"p50", 0.5}, {"p95", 0.95}, {"p99", 0.99}} {
		if n := len(c.latencies); n > 0 {
			out[q.name] = c.latencies[min(int(float64(n)*q.p), n-1)].String()
		}
	}
	return out
}

func run(ctx context.Context, o runOptions) (*result, error) {
	c := &client{base: o.api, key: o.key, http: &http.Client{Timeout: 10 * time.Second}}
	var replays, disputes, transitions atomic.Int64
	var ids opened
	started := time.Now()
	ticker := time.NewTicker(time.Duration(float64(time.Second) / o.rps))
	defer ticker.Stop()
	deadline := time.After(o.d)
	var wg sync.WaitGroup

	// the workbench loop runs beside the API loop: a browser reads the workbench while systems write to the API
	var wb *client
	pageTick := make(<-chan time.Time)
	if o.workbench != "" && o.session != "" {
		wb = &client{base: o.workbench, http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
		t := time.NewTicker(time.Duration(float64(time.Second) / o.pages))
		defer t.Stop()
		pageTick = t.C
	}
	pagePaths := []string{"/otp", "/otp?state=INITIATED", "/otp/keys", "/otp/templates"}
loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-ticker.C:
			wg.Add(1)
			go func() { defer wg.Done(); iteration(ctx, c, &replays, &disputes, &transitions, &ids) }()
		case <-pageTick:
			path := pagePaths[rand.IntN(len(pagePaths))]         //nolint:gosec // load mix, not a secret
			if id := ids.pick(); id != "" && rand.IntN(2) == 0 { //nolint:gosec // load mix, not a secret
				path = "/otp/disputes/" + id
			}
			wg.Add(1)
			go func() { defer wg.Done(); page(ctx, wb, o.session, path) }()
		}
	}
	wg.Wait()
	ended := time.Now()
	prom, rps := o.prom, o.rps

	res := &result{Started: started, Ended: ended, TargetRPS: rps, Statuses: map[string]int64{},
		Replays: replays.Load(), Disputes: disputes.Load(), Transitions: transitions.Load(), ClientLatency: map[string]string{}}
	res.Requests = c.requests.Load()
	res.AchievedRPS = float64(res.Requests) / ended.Sub(started).Seconds()
	c.statuses.Range(func(k, v any) bool { res.Statuses[k.(string)] = v.(*atomic.Int64).Load(); return true })
	res.ClientLatency = percentiles(c)

	// Server-side view of the same window, from the metrics the dashboard uses.
	window := fmt.Sprintf("%ds", int(ended.Sub(started).Seconds())+15)
	res.Latency = map[string]any{}
	res.Problems = map[string]any{}
	for _, q := range []struct{ name, quantile string }{{"p50", "0.5"}, {"p95", "0.95"}, {"p99", "0.99"}} {
		v, err := query(ctx, prom, fmt.Sprintf(`histogram_quantile(%s, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{service_name="dispute-engine", http_route!=""}[%s])))`, q.quantile, window), ended)
		if err != nil {
			res.Notes = append(res.Notes, "prometheus latency: "+err.Error())
			break
		}
		res.Latency[q.name] = v
	}
	v, err := query(ctx, prom, fmt.Sprintf(`sum by (code) (increase(http_server_problems_total[%s]))`, window), ended)
	if err != nil {
		res.Notes = append(res.Notes, "prometheus problems: "+err.Error())
	} else {
		res.Problems = v
	}
	// what the machines themselves did during the window: the reason a latency number looks the way it does
	res.Runtime = map[string]any{}
	for name, expr := range map[string]string{
		"apiGoroutinesMax":          fmt.Sprintf(`max_over_time(go_goroutine_count{service_name="dispute-engine"}[%s])`, window),
		"apiHeapUsedMaxBytes":       fmt.Sprintf(`max_over_time(go_memory_used_bytes{service_name="dispute-engine"}[%s])`, window),
		"apiPostgresP95Seconds":     fmt.Sprintf(`histogram_quantile(0.95, sum by (le) (rate(db_client_operation_duration_seconds_bucket{service_name="dispute-engine"}[%s])))`, window),
		"workbenchEventLoopP99Max":  fmt.Sprintf(`max_over_time(nodejs_eventloop_delay_p99_seconds{service_name="dispute-workbench"}[%s])`, window),
		"workbenchEventLoopUtilMax": fmt.Sprintf(`max_over_time(nodejs_eventloop_utilization_ratio{service_name="dispute-workbench"}[%s])`, window),
		"workbenchHeapUsedMaxBytes": fmt.Sprintf(`sum(max_over_time(v8js_memory_heap_used_bytes{service_name="dispute-workbench"}[%s]))`, window),
		"outboxBacklogMax":          fmt.Sprintf(`max_over_time(dispute_outbox_backlog[%s])`, window),
		"noticeDeliveryDelayP95":    fmt.Sprintf(`histogram_quantile(0.95, sum by (le) (rate(dispute_notice_delivery_delay_seconds_bucket[%s])))`, window),
	} {
		if v, err := query(ctx, prom, expr, ended); err == nil {
			if val, ok := v["all"]; ok {
				res.Runtime[name] = val
			}
		}
	}
	if wb != nil {
		w := &workbenchResult{TargetPagesPerSecond: o.pages, Pages: wb.requests.Load(), Statuses: map[string]int64{}, ClientLatency: percentiles(wb)}
		wb.statuses.Range(func(k, v any) bool { w.Statuses[k.(string)] = v.(*atomic.Int64).Load(); return true })
		if v, err := query(ctx, prom, fmt.Sprintf(`histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{service_name="dispute-workbench"}[%s])))`, window), ended); err == nil {
			w.PageLatency = v
		}
		if v, err := queryBy(ctx, prom, "workbench_server_fn", fmt.Sprintf(`histogram_quantile(0.95, sum by (le, workbench_server_fn) (rate(workbench_server_fn_duration_seconds_bucket{service_name="dispute-workbench"}[%s])))`, window), ended); err == nil {
			w.ServerFnLatency = v
		}
		res.Workbench = w
	}
	return res, nil
}

// query runs an instant query and flattens the result into label-string -> value, keyed by route or code.
func query(ctx context.Context, prom, expr string, at time.Time) (map[string]any, error) {
	return queryBy(ctx, prom, "", expr, at)
}

// queryBy is query keyed by one named label first.
func queryBy(ctx context.Context, prom, label, expr string, at time.Time) (map[string]any, error) {
	u := prom + "/api/v1/query?" + url.Values{"query": {expr}, "time": {fmt.Sprint(at.Unix())}}.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body
	var body struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, r := range body.Data.Result {
		key := r.Metric[label]
		if key == "" {
			key = r.Metric["http_route"]
		}
		if key == "" {
			key = r.Metric["code"]
		}
		label := key
		if label == "" {
			label = "all"
		}
		// Routes with no traffic in the window come back as NaN; leave them out rather than write NaN into the record.
		if len(r.Value) == 2 && r.Value[1] != "NaN" {
			out[label] = r.Value[1]
		}
	}
	return out, nil
}
