package tlg

import "github.com/gotd/td/tg"

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
