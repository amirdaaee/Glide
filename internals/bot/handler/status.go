package bot

import "github.com/amirdaaee/Glide/internals/domain"

// ClientMediaStatus is the user-facing media status sent in Telegram replies.
type ClientMediaStatus string

const (
	// ClientStatusRejected is sent when the bot will not accept the media.
	ClientStatusRejected ClientMediaStatus = "rejected"
	// ClientStatusProcessing is sent while ingest is in progress.
	ClientStatusProcessing ClientMediaStatus = "processing"
	// ClientStatusFailed is sent when ingest fails.
	ClientStatusFailed ClientMediaStatus = "failed"
	// ClientStatusSuccess is sent when media is ready.
	ClientStatusSuccess ClientMediaStatus = "success"
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

// terminal reports whether the status is success or failed.
func (s ClientMediaStatus) terminal() bool {
	return s == ClientStatusSuccess || s == ClientStatusFailed
}
