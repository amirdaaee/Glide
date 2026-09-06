package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	natsio "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Client struct {
	nc         *natsio.Conn
	js         natsio.JetStreamContext
	stream     string
	ackWait    time.Duration
	maxDeliver int
	ll         *zap.Logger
}

var _ pipeline.IPublisher = (*Client)(nil)
var _ pipeline.ISubscriber = (*Client)(nil)

func (c *Client) PublishWork(ctx context.Context, subject string, msg pipeline.WorkMsg) error {
	return c.publish(ctx, subject, jetStreamMsgID(subject, msg.TaskID), msg)
}

func (c *Client) PublishResult(ctx context.Context, subject string, msg pipeline.ResultMsg) error {
	return c.publish(ctx, subject, jetStreamMsgID(subject, msg.TaskID), msg)
}

func (c *Client) publish(ctx context.Context, subject, msgID string, payload any) error {
	ll := c.ll.Named("publish").With(zap.String("subject", subject), zap.String("msg_id", msgID))
	data, err := json.Marshal(payload)
	if err != nil {
		ll.Error("can not marshal nats payload", zap.Error(err))
		return fmt.Errorf("can not marshal nats payload: %w", err)
	}
	m := &natsio.Msg{
		Subject: subject,
		Data:    data,
		Header:  natsio.Header{},
	}
	if msgID != "" {
		m.Header.Set(natsio.MsgIdHdr, msgID)
	}
	ack, err := c.js.PublishMsg(m, natsio.Context(ctx))
	if err != nil {
		ll.Error("can not publish", zap.Error(err), zap.Int("bytes", len(data)))
		return fmt.Errorf("can not publish to %s: %w", subject, err)
	}
	if ack != nil && ack.Duplicate {
		// Dedup is stream-wide. A reused Nats-Msg-Id is acknowledged but not stored,
		// so no consumer (e.g. the orchestrator) will ever see the message.
		ll.Warn("jetstream dropped duplicate message", zap.Uint64("seq", ack.Sequence), zap.Int("bytes", len(data)))
		return nil
	}
	ll.Debug("published", zap.Int("bytes", len(data)))
	return nil
}

func (c *Client) SubscribeWork(ctx context.Context, subject, group string, h pipeline.WorkHandler) error {
	return c.subscribe(ctx, subject, group, func(ctx context.Context, data []byte) error {
		var msg pipeline.WorkMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			c.ll.Named("SubscribeWork").Error("can not unmarshal work message",
				zap.Error(err),
				zap.String("subject", subject),
				zap.Int("bytes", len(data)),
			)
			return err
		}
		return h(ctx, msg)
	})
}

func (c *Client) SubscribeResult(ctx context.Context, subject, group string, h pipeline.ResultHandler) error {
	return c.subscribe(ctx, subject, group, func(ctx context.Context, data []byte) error {
		var msg pipeline.ResultMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			c.ll.Named("SubscribeResult").Error("can not unmarshal result message",
				zap.Error(err),
				zap.String("subject", subject),
				zap.Int("bytes", len(data)),
			)
			return err
		}
		return h(ctx, msg)
	})
}

func (c *Client) subscribe(ctx context.Context, subject, group string, handle func(context.Context, []byte) error) error {
	ll := c.ll.Named("subscribe").With(
		zap.String("subject", subject),
		zap.String("group", group),
	)
	durable := durableName(group, subject)
	ll.Info("subscribing", zap.String("durable", durable), zap.String("stream", c.stream))
	sub, err := c.js.QueueSubscribe(subject, group, func(m *natsio.Msg) {
		ll := ll.With(zap.Int("bytes", len(m.Data)))
		if err := handle(ctx, m.Data); err != nil {
			ll.Warn("handler failed", zap.Error(err))
			if nakErr := m.NakWithDelay(time.Second); nakErr != nil {
				ll.Error("can not nak message", zap.Error(nakErr))
			} else {
				ll.Debug("nacked message")
			}
			return
		}
		if ackErr := m.Ack(); ackErr != nil {
			ll.Error("can not ack message", zap.Error(ackErr))
			return
		}
		ll.Debug("acked message")
	}, natsio.ManualAck(), natsio.AckWait(c.ackWait), natsio.MaxDeliver(c.maxDeliver), natsio.Durable(durable), natsio.BindStream(c.stream), natsio.Context(ctx))
	if err != nil {
		ll.Error("can not subscribe", zap.Error(err))
		return fmt.Errorf("can not subscribe to %s: %w", subject, err)
	}
	<-ctx.Done()
	ll.Info("draining subscription")
	if err := sub.Drain(); err != nil {
		ll.Error("can not drain subscription", zap.Error(err))
		return fmt.Errorf("can not drain subscription %s: %w", subject, err)
	}
	ll.Info("subscription stopped")
	return nil
}

func (c *Client) Close() {
	if c.nc != nil {
		c.ll.Info("closing nats connection")
		c.nc.Close()
	}
}

func durableName(group, subject string) string {
	s := group + "-" + subject
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, ">", "all")
	return s
}

// jetStreamMsgID scopes Nats-Msg-Id by subject. JetStream dedup is stream-wide,
// so work and result for the same task must not share an id.
func jetStreamMsgID(subject, taskID string) string {
	if taskID == "" {
		return ""
	}
	return subject + ":" + taskID
}

func ensureStream(js natsio.JetStreamContext, name string) error {
	cfg := &natsio.StreamConfig{
		Name:      name,
		Subjects:  []string{pipeline.SubjectPrefix},
		Retention: natsio.WorkQueuePolicy,
	}
	_, err := js.StreamInfo(name)
	if err == nil {
		return nil
	}
	if !errors.Is(err, natsio.ErrStreamNotFound) {
		return fmt.Errorf("can not get stream info: %w", err)
	}
	if _, err := js.AddStream(cfg); err != nil {
		return fmt.Errorf("can not create stream %s: %w", name, err)
	}
	return nil
}

func NewClient(cfg *config.NatsConfigType) (*Client, error) {
	ll := log.GetLogger(log.PIPELINE).Named("nats")
	stream := cfg.Stream
	if stream == "" {
		stream = pipeline.StreamName
	}
	ll.Info("connecting", zap.String("stream", stream))
	nc, err := natsio.Connect(cfg.URL)
	if err != nil {
		ll.Error("can not connect to nats", zap.Error(err))
		return nil, fmt.Errorf("can not connect to nats: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		ll.Error("can not create jetstream context", zap.Error(err))
		return nil, fmt.Errorf("can not create jetstream context: %w", err)
	}
	if err := ensureStream(js, stream); err != nil {
		nc.Close()
		ll.Error("can not ensure stream", zap.Error(err), zap.String("stream", stream))
		return nil, err
	}
	ll.Info("connected", zap.String("stream", stream), zap.Duration("ack_wait", cfg.AckWait), zap.Int("max_deliver", cfg.MaxDeliver))
	return &Client{
		nc:         nc,
		js:         js,
		stream:     stream,
		ackWait:    cfg.AckWait,
		maxDeliver: cfg.MaxDeliver,
		ll:         ll,
	}, nil
}
