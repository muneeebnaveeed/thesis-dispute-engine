package application

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/telemetry"
)

// Mail is one message to a customer; Text is the body, HTML the same words laid out, Files what travels with it.
type Mail struct {
	To      string
	Subject string
	Text    string
	HTML    string
	Files   []MailFile
}

// MailFile is one attachment as the mailer needs it.
type MailFile struct {
	Name        string
	ContentType string
	Content     []byte
}

// Mailer delivers mail; an error means not delivered and the notice stays in the outbox for another attempt.
type Mailer interface {
	Send(ctx context.Context, m Mail) error
}

// Dispatcher drains the notice outbox: on a nudge from a transition and on a timer, so nothing waits long and
// nothing is lost if the nudge is. Every replica may run one; the store hands each notice to one of them.
type Dispatcher struct {
	store  Store
	mailer Mailer
	render func(NoticeRecord) (Mail, error)
	log    *slog.Logger
	kick   chan struct{}
	every  time.Duration
}

// NewDispatcher wires a dispatcher; render turns a stored notice into the mail to send.
func NewDispatcher(store Store, mailer Mailer, render func(NoticeRecord) (Mail, error), log *slog.Logger) *Dispatcher {
	return &Dispatcher{store: store, mailer: mailer, render: render, log: log, kick: make(chan struct{}, 1), every: 15 * time.Second}
}

// Kick asks for a pass soon; it never blocks and coalesces with a pending kick.
func (d *Dispatcher) Kick() {
	select {
	case d.kick <- struct{}{}:
	default:
	}
}

// deliveryMeters are the dispatcher's instruments: what was handed to the mailer, how long each email waited in
// the outbox, and how many wait right now.
type deliveryMeters struct {
	sent  metric.Int64Counter
	delay metric.Float64Histogram
}

func (d *Dispatcher) meters() (deliveryMeters, error) {
	m := otel.Meter(scopeName)
	sent, err := m.Int64Counter("dispute.notices_sent", metric.WithDescription("Notice emails handed to the mailer, by outcome"))
	if err != nil {
		return deliveryMeters{}, err
	}
	delay, err := m.Float64Histogram("dispute.notice_delivery_delay", metric.WithUnit("s"),
		metric.WithDescription("Seconds between a notice being queued and the mailer accepting it"))
	if err != nil {
		return deliveryMeters{}, err
	}
	_, err = m.Int64ObservableGauge("dispute.outbox_backlog", metric.WithDescription("Emails queued and not yet sent, read from the store on each scrape"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			n, err := d.store.OutboxBacklog(ctx)
			if err != nil {
				return err
			}
			o.Observe(n)
			return nil
		}))
	if err != nil {
		return deliveryMeters{}, err
	}
	return deliveryMeters{sent: sent, delay: delay}, nil
}

// Run delivers until ctx ends.
func (d *Dispatcher) Run(ctx context.Context) {
	meters, err := d.meters()
	if err != nil {
		d.log.Error("notice dispatcher: meter", "err", err)
		return
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-d.kick:
		}
		for {
			n := d.pass(ctx, meters)
			if n == 0 {
				break
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(d.every)
	}
}

// pass claims one batch and attempts each; it returns how many it claimed.
func (d *Dispatcher) pass(ctx context.Context, meters deliveryMeters) int {
	batch, err := d.store.ClaimNotices(ctx, 20)
	if err != nil {
		if ctx.Err() == nil {
			d.log.Warn("notice dispatcher: claim", "err", err)
		}
		return 0
	}
	for _, n := range batch {
		d.deliver(ctx, n, meters)
	}
	return len(batch)
}

// deliver is one attempt at one notice, as its own trace linked to the request that queued it: the delivery is
// asynchronous work, so a child span would end long after its parent and a link is the honest relation.
func (d *Dispatcher) deliver(ctx context.Context, n NoticeRecord, meters deliveryMeters) {
	opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(
		attribute.String("notice.kind", string(n.Kind)),
		attribute.Int("notice.attempt", n.Attempts),
		attribute.Bool("notice.resend", n.ResendOf != nil),
	)}
	if link, ok := telemetry.LinkTo(n.TraceContext); ok {
		opts = append(opts, trace.WithLinks(link))
	}
	ctx, span := otel.Tracer(scopeName).Start(ctx, "notice.deliver", opts...)
	defer span.End()
	{
		outcome := "sent"
		failure := ""
		m, err := d.render(n)
		if err == nil {
			// A resend carries what the original carried.
			var files []Attachment
			source := n.ID
			if n.ResendOf != nil {
				source = *n.ResendOf
			}
			if files, err = d.store.NoticeAttachments(ctx, source); err == nil {
				for _, f := range files {
					m.Files = append(m.Files, MailFile{Name: f.Filename, ContentType: f.ContentType, Content: f.Content})
				}
			}
		}
		if err != nil {
			failure, outcome = "render: "+err.Error(), "render-failed"
		} else if err := d.mailer.Send(ctx, m); err != nil {
			failure, outcome = err.Error(), "failed"
		}
		if err := d.store.FinishNotice(ctx, n.ID, failure); err != nil && ctx.Err() == nil {
			d.log.Warn("notice dispatcher: finish", "id", n.ID, "err", err)
		}
		by := metric.WithAttributes(attribute.String("outcome", outcome), attribute.String("kind", string(n.Kind)))
		meters.sent.Add(ctx, 1, by)
		span.SetAttributes(attribute.String("notice.outcome", outcome))
		if failure != "" {
			span.SetStatus(codes.Error, failure)
			d.log.Warn("notice not delivered", "id", n.ID, "tenant", n.TenantID, "dispute", n.DisputeID, "kind", n.Kind, "attempt", n.Attempts, "err", failure)
		} else {
			meters.delay.Record(ctx, time.Since(n.CreatedAt).Seconds(), by)
			d.log.Info("notice sent", "id", n.ID, "tenant", n.TenantID, "dispute", n.DisputeID, "kind", n.Kind, "to", n.Recipient)
		}
	}
}
