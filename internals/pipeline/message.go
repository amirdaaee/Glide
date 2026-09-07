package pipeline

import (
	"encoding/json"

	"github.com/amirdaaee/Glide/internals/domain"
)

const (
	SubjectWorkIngest     = "glide.media.v1.work.ingest"
	SubjectWorkDownload   = "glide.media.v1.work.download"
	SubjectResultIngest   = "glide.media.v1.result.ingest"
	SubjectResultDownload = "glide.media.v1.result.download"
	SubjectDLQ            = "glide.media.v1.dlq"
	StreamName            = "GLIDE_MEDIA"
	SubjectPrefix         = "glide.media.v1.>"
)

type WorkMsg struct {
	TaskID  string          `json:"task_id"`
	MediaID string          `json:"media_id"`
	Step    string          `json:"step"`
	Attempt int             `json:"attempt"`
	Payload json.RawMessage `json:"payload"`
}

type ResultMsg struct {
	TaskID  string          `json:"task_id"`
	MediaID string          `json:"media_id"`
	Step    string          `json:"step"`
	Attempt int             `json:"attempt"`
	OK      bool            `json:"ok"`
	Error   *StepError      `json:"error,omitempty"`
	Output  json.RawMessage `json:"output,omitempty"`
}

type StepError struct {
	Message   string `json:"message"`
	Permanent bool   `json:"permanent"`
}

type IngestPayload struct {
	ChannelID int64 `json:"channel_id"`
	MessageID int   `json:"message_id"`
	FileID    int64 `json:"file_id"`
}

type DownloadPayload struct {
	ChannelID int64  `json:"channel_id"`
	MessageID int    `json:"message_id"`
	FileID    int64  `json:"file_id"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
}

type IngestOutput struct {
	ThumbnailURL string `json:"thumbnail_url"`
}

type DownloadOutput struct {
	Byse *domain.ByseFile `json:"byse,omitempty"`
	Size int64            `json:"size"`
}

func WorkSubject(step domain.JobStep) string {
	switch step {
	case domain.JobStepIngest:
		return SubjectWorkIngest
	case domain.JobStepDownload:
		return SubjectWorkDownload
	default:
		return "glide.media.v1.work." + string(step)
	}
}

func ResultSubject(step domain.JobStep) string {
	switch step {
	case domain.JobStepIngest:
		return SubjectResultIngest
	case domain.JobStepDownload:
		return SubjectResultDownload
	default:
		return "glide.media.v1.result." + string(step)
	}
}
