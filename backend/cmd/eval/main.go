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
}

func main() {
	api := flag.String("api", "http://localhost:8090", "API base URL")
	key := flag.String("key", "tk_dev_tenant_a", "tenant key")
	prom := flag.String("prometheus", "http://localhost:9090", "Prometheus base URL")
	rps := flag.Float64("rps", 5, "lifecycle iterations per second (each is 4 to 5 requests)")
	seconds := flag.Int("seconds", 120, "run length")
	out := flag.String("out", "load.json", "output file")
	flag.Parse()

	res, err := run(context.Background(), *api, *key, *prom, *rps, time.Duration(*seconds)*time.Second)
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
func iteration(ctx context.Context, c *client, replays, disputes, transitions *atomic.Int64) {
	txn := transactions[rand.IntN(len(transactions))] //nolint:gosec // load mix, not a secret
	idem := uuid.NewString()
	body := map[string]any{"transactionId": txn, "actor": "eval"}
	code, d := c.do(ctx, http.MethodPost, "/disputes", body, idem)
	if code != 201 {
		return
	}
	disputes.Add(1)
	id, _ := d["id"].(string)
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

func run(ctx context.Context, api, key, prom string, rps float64, d time.Duration) (*result, error) {
	c := &client{base: api, key: key, http: &http.Client{Timeout: 10 * time.Second}}
	var replays, disputes, transitions atomic.Int64
	started := time.Now()
	ticker := time.NewTicker(time.Duration(float64(time.Second) / rps))
	defer ticker.Stop()
	deadline := time.After(d)
	var wg sync.WaitGroup
loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-ticker.C:
			wg.Add(1)
			go func() { defer wg.Done(); iteration(ctx, c, &replays, &disputes, &transitions) }()
		}
	}
	wg.Wait()
	ended := time.Now()

	res := &result{Started: started, Ended: ended, TargetRPS: rps, Statuses: map[string]int64{},
		Replays: replays.Load(), Disputes: disputes.Load(), Transitions: transitions.Load(), ClientLatency: map[string]string{}}
	res.Requests = c.requests.Load()
	res.AchievedRPS = float64(res.Requests) / ended.Sub(started).Seconds()
	c.statuses.Range(func(k, v any) bool { res.Statuses[k.(string)] = v.(*atomic.Int64).Load(); return true })
	c.mu.Lock()
	sort.Slice(c.latencies, func(i, j int) bool { return c.latencies[i] < c.latencies[j] })
	for _, q := range []struct {
		name string
		p    float64
	}{{"p50", 0.5}, {"p95", 0.95}, {"p99", 0.99}} {
		if n := len(c.latencies); n > 0 {
			res.ClientLatency[q.name] = c.latencies[min(int(float64(n)*q.p), n-1)].String()
		}
	}
	c.mu.Unlock()

	// Server-side view of the same window, from the metrics the dashboard uses.
	window := fmt.Sprintf("%ds", int(ended.Sub(started).Seconds())+15)
	res.Latency = map[string]any{}
	res.Problems = map[string]any{}
	for _, q := range []string{"0.5", "0.95", "0.99"} {
		v, err := query(ctx, prom, fmt.Sprintf(`histogram_quantile(%s, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{http_route!=""}[%s])))`, q, window), ended)
		if err != nil {
			res.Notes = append(res.Notes, "prometheus latency: "+err.Error())
			break
		}
		res.Latency["p"+q[2:]] = v
	}
	v, err := query(ctx, prom, fmt.Sprintf(`sum by (code) (increase(http_server_problems_total[%s]))`, window), ended)
	if err != nil {
		res.Notes = append(res.Notes, "prometheus problems: "+err.Error())
	} else {
		res.Problems = v
	}
	return res, nil
}

// query runs an instant query and flattens the result into label-string -> value.
func query(ctx context.Context, prom, expr string, at time.Time) (map[string]any, error) {
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
		label := r.Metric["http_route"]
		if label == "" {
			label = r.Metric["code"]
		}
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
