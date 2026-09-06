package tlg

import (
	"context"
	"fmt"
	"sync"

	"github.com/gotd/td/tg"
)

// BareChannelID converts a Bot API-style channel id (-100…) to an MTProto channel id.
func BareChannelID(id int64) int64 {
	if id >= 0 {
		return id
	}
	id = -id
	const prefix = int64(1_000_000_000_000) // 10^12
	if id >= prefix {
		return id % prefix
	}
	return id
}

// ChannelInputPeer builds an InputPeerChannel from config values.
func ChannelInputPeer(channelID, accessHash int64) *tg.InputPeerChannel {
	return &tg.InputPeerChannel{
		ChannelID:  BareChannelID(channelID),
		AccessHash: accessHash,
	}
}

// ChannelInput builds an InputChannel from config values.
func ChannelInput(channelID, accessHash int64) *tg.InputChannel {
	return &tg.InputChannel{
		ChannelID:  BareChannelID(channelID),
		AccessHash: accessHash,
	}
}

// Channel resolves a storage channel's access hash once per Telegram account and keeps it in memory.
type Channel struct {
	id       int64
	mu       sync.Mutex
	resolved bool
	hash     int64
}

// NewChannel creates a Channel from a Bot API-style or bare channel id.
func NewChannel(id int64) *Channel {
	return &Channel{id: BareChannelID(id)}
}

// Resolve fetches the channel access hash via channels.getChannels (access_hash 0) if not already cached.
func (c *Channel) Resolve(ctx context.Context, api *tg.Client) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resolved {
		return nil
	}
	if api == nil {
		return fmt.Errorf("telegram client is not ready")
	}
	res, err := api.ChannelsGetChannels(ctx, []tg.InputChannelClass{
		&tg.InputChannel{ChannelID: c.id},
	})
	if err != nil {
		return fmt.Errorf("can not get channel %d: %w", c.id, err)
	}
	for _, chat := range res.GetChats() {
		ch, ok := chat.(*tg.Channel)
		if !ok || ch.ID != c.id {
			continue
		}
		c.hash = ch.AccessHash
		c.resolved = true
		return nil
	}
	return fmt.Errorf("channel %d not found", c.id)
}

// InputPeer returns an InputPeerChannel after resolving the access hash.
func (c *Channel) InputPeer(ctx context.Context, api *tg.Client) (*tg.InputPeerChannel, error) {
	if err := c.Resolve(ctx, api); err != nil {
		return nil, err
	}
	c.mu.Lock()
	hash := c.hash
	c.mu.Unlock()
	return ChannelInputPeer(c.id, hash), nil
}

// Input returns an InputChannel after resolving the access hash.
func (c *Channel) Input(ctx context.Context, api *tg.Client) (*tg.InputChannel, error) {
	if err := c.Resolve(ctx, api); err != nil {
		return nil, err
	}
	c.mu.Lock()
	hash := c.hash
	c.mu.Unlock()
	return ChannelInput(c.id, hash), nil
}
