package pipeline

import (
	"encoding/json"

	"github.com/amirdaaee/Glide/internals/domain"
)

const (
	// SubjectWorkIngest is the NATS subject for ingest work.
	SubjectWorkIngest = "glide.media.v1.work.ingest"
	// SubjectWorkDownload is the NATS subject for download work.
	SubjectWorkDownload = "glide.media.v1.work.download"
	// SubjectWorkNotify is the NATS subject for notify work.
	SubjectWorkNotify = "glide.media.v1.work.notify"
	// SubjectResultIngest is the NATS subject for ingest results.
	SubjectResultIngest = "glide.media.v1.result.ingest"
	// SubjectResultDownload is the NATS subject for download results.
	SubjectResultDownload = "glide.media.v1.result.download"
	// SubjectDLQ is the NATS dead-letter subject.
	SubjectDLQ = "glide.media.v1.dlq"
	// StreamName is the default JetStream stream name.
	StreamName = "GLIDE_MEDIA"
	// SubjectPrefix is the JetStream subject filter for the stream.
	SubjectPrefix = "glide.media.v1.>"
)

// WorkMsg is a pipeline work item published to NATS.
type WorkMsg struct {
	TaskID  string          `json:"task_id"`
	MediaID string          `json:"media_id"`
	Step    string          `json:"step"`
	Attempt int             `json:"attempt"`
	Payload json.RawMessage `json:"payload"`
}

// ResultMsg is a pipeline step result published to NATS.
type ResultMsg struct {
	TaskID  string          `json:"task_id"`
	MediaID string          `json:"media_id"`
	Step    string          `json:"step"`
	Attempt int             `json:"attempt"`
	OK      bool            `json:"ok"`
	Error   *StepError      `json:"error,omitempty"`
	Output  json.RawMessage `json:"output,omitempty"`
}

// StepError describes a failed pipeline step.
type StepError struct {
	Message   string `json:"message"`
	Permanent bool   `json:"permanent"`
}

// IngestPayload is work payload for the ingest step.
type IngestPayload struct {
	ChannelID int64 `json:"channel_id"`
	MessageID int   `json:"message_id"`
	FileID    int64 `json:"file_id"`
}

// DownloadPayload is work payload for the download step.
type DownloadPayload struct {
	ChannelID int64  `json:"channel_id"`
	MessageID int    `json:"message_id"`
	FileID    int64  `json:"file_id"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
}

// IngestOutput is result payload from the ingest step.
type IngestOutput struct {
	ThumbnailURL string `json:"thumbnail_url"`
}

// DownloadOutput is result payload from the download step.
type DownloadOutput struct {
	Byse *domain.ByseFile `json:"byse,omitempty"`
	Size int64            `json:"size"`
}

// NotifyPayload is work payload for the notify step.
type NotifyPayload struct {
	Status domain.MediaStatus `json:"status"`
}

// WorkSubject returns the NATS work subject for a job step.
func WorkSubject(step domain.JobStep) string {
	switch step {
	case domain.JobStepIngest:
		return SubjectWorkIngest
	case domain.JobStepDownload:
		return SubjectWorkDownload
	case domain.JobStepNotify:
		return SubjectWorkNotify
	default:
		return "glide.media.v1.work." + string(step)
	}
}

// ResultSubject returns the NATS result subject for a job step.
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
