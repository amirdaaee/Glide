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
	return c.publish(ctx, subject, msg.TaskID, msg)
}

func (c *Client) PublishResult(ctx context.Context, subject string, msg pipeline.ResultMsg) error {
	return c.publish(ctx, subject, msg.TaskID, msg)
}

func (c *Client) publish(ctx context.Context, subject, msgID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
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
	if _, err := c.js.PublishMsg(m, natsio.Context(ctx)); err != nil {
		return fmt.Errorf("can not publish to %s: %w", subject, err)
	}
	return nil
}

func (c *Client) SubscribeWork(ctx context.Context, subject, group string, h pipeline.WorkHandler) error {
	return c.subscribe(ctx, subject, group, func(ctx context.Context, data []byte) error {
		var msg pipeline.WorkMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		return h(ctx, msg)
	})
}

func (c *Client) SubscribeResult(ctx context.Context, subject, group string, h pipeline.ResultHandler) error {
	return c.subscribe(ctx, subject, group, func(ctx context.Context, data []byte) error {
		var msg pipeline.ResultMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		return h(ctx, msg)
	})
}

func (c *Client) subscribe(ctx context.Context, subject, group string, handle func(context.Context, []byte) error) error {
	durable := durableName(group, subject)
	sub, err := c.js.QueueSubscribe(subject, group, func(m *natsio.Msg) {
		if err := handle(ctx, m.Data); err != nil {
			c.ll.With(zap.Error(err), zap.String("subject", subject)).Warn("handler failed")
			if nakErr := m.NakWithDelay(time.Second); nakErr != nil {
				c.ll.With(zap.Error(nakErr)).Error("can not nak message")
			}
			return
		}
		if ackErr := m.Ack(); ackErr != nil {
			c.ll.With(zap.Error(ackErr)).Error("can not ack message")
		}
	}, natsio.ManualAck(), natsio.AckWait(c.ackWait), natsio.MaxDeliver(c.maxDeliver), natsio.Durable(durable), natsio.BindStream(c.stream), natsio.Context(ctx))
	if err != nil {
		return fmt.Errorf("can not subscribe to %s: %w", subject, err)
	}
	<-ctx.Done()
	if err := sub.Drain(); err != nil {
		return fmt.Errorf("can not drain subscription %s: %w", subject, err)
	}
	return nil
}

func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

func durableName(group, subject string) string {
	s := group + "-" + subject
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, ">", "all")
	return s
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
	stream := cfg.Stream
	if stream == "" {
		stream = pipeline.StreamName
	}
	nc, err := natsio.Connect(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("can not connect to nats: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("can not create jetstream context: %w", err)
	}
	if err := ensureStream(js, stream); err != nil {
		nc.Close()
		return nil, err
	}
	return &Client{
		nc:         nc,
		js:         js,
		stream:     stream,
		ackWait:    cfg.AckWait,
		maxDeliver: cfg.MaxDeliver,
		ll:         log.GetLogger(log.PIPELINE).Named("nats"),
	}, nil
}
