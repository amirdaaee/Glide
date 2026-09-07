package bot

import "github.com/amirdaaee/Glide/internals/domain"

// ClientMediaStatus is the user-facing media status sent in Telegram replies.
type ClientMediaStatus string

const (
	ClientStatusRejected   ClientMediaStatus = "rejected"
	ClientStatusProcessing ClientMediaStatus = "processing"
	ClientStatusFailed     ClientMediaStatus = "failed"
	ClientStatusSuccess    ClientMediaStatus = "success"
)

// ClientStatusFromMedia maps a stored media status to the client-facing label.
func ClientStatusFromMedia(status domain.MediaStatus) ClientMediaStatus {
	switch status {
	case domain.MediaStatusReady:
		return ClientStatusSuccess
	case domain.MediaStatusFailed:
		return ClientStatusFailed
	default:
		return ClientStatusProcessing
	}
}

func (s ClientMediaStatus) terminal() bool {
	return s == ClientStatusSuccess || s == ClientStatusFailed
}
