package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/pipeline"
)

type IIngestDispatcher interface {
	Dispatch(ctx context.Context, media *domain.MediaFile) error
}

type IngestDispatcher struct {
	jobs              IJobService
	pub               pipeline.IPublisher
	channelID         int64
	channelAccessHash int64
}

var _ IIngestDispatcher = (*IngestDispatcher)(nil)

func (d *IngestDispatcher) Dispatch(ctx context.Context, media *domain.MediaFile) error {
	if media == nil || media.ID.IsZero() {
		return fmt.Errorf("media id is required")
	}
	if media.HasByse() {
		return nil
	}
	if media.ThumbnailURL != "" {
		return d.dispatchDownload(ctx, media)
	}
	job, err := d.jobs.GetByMediaID(ctx, media.ID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		job = &domain.MediaJob{
			MediaID:        media.ID,
			Step:           domain.JobStepIngest,
			StatusVer:      0,
			IdempotencyKey: media.ID.Hex(),
		}
		if err := d.jobs.Create(ctx, job); err != nil {
			if !errors.Is(err, domain.ErrAlreadyExists) {
				return err
			}
		}
	} else if job.Step != domain.JobStepIngest {
		return nil
	}
	payload, err := json.Marshal(pipeline.IngestPayload{
		ChannelID:         d.channelID,
		ChannelAccessHash: d.channelAccessHash,
		MessageID:         media.MessageID,
		FileID:            media.Meta.FileID,
	})
	if err != nil {
		return fmt.Errorf("can not marshal ingest payload: %w", err)
	}
	return d.pub.PublishWork(ctx, pipeline.WorkSubject(domain.JobStepIngest), pipeline.WorkMsg{
		TaskID:  media.ID.Hex() + "-ingest",
		MediaID: media.ID.Hex(),
		Step:    string(domain.JobStepIngest),
		Attempt: 1,
		Payload: payload,
	})
}

func (d *IngestDispatcher) dispatchDownload(ctx context.Context, media *domain.MediaFile) error {
	job, err := d.jobs.GetByMediaID(ctx, media.ID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		job = &domain.MediaJob{
			MediaID:        media.ID,
			Step:           domain.JobStepDownload,
			StatusVer:      0,
			IdempotencyKey: media.ID.Hex(),
		}
		if err := d.jobs.Create(ctx, job); err != nil {
			if !errors.Is(err, domain.ErrAlreadyExists) {
				return err
			}
		}
	} else if job.Step == domain.JobStepIngest {
		if _, err := d.jobs.Transition(ctx, media.ID, domain.JobStepIngest, domain.JobStepDownload, job.StatusVer); err != nil {
			return err
		}
	} else if job.Step != domain.JobStepDownload {
		return nil
	}
	return PublishDownloadWork(ctx, d.pub, media, d.channelID, d.channelAccessHash)
}

func PublishDownloadWork(ctx context.Context, pub pipeline.IPublisher, media *domain.MediaFile, channelID, channelAccessHash int64) error {
	if pub == nil {
		return fmt.Errorf("publisher is nil")
	}
	if media == nil || media.ID.IsZero() {
		return fmt.Errorf("media id is required")
	}
	payload, err := json.Marshal(pipeline.DownloadPayload{
		ChannelID:         channelID,
		ChannelAccessHash: channelAccessHash,
		MessageID:         media.MessageID,
		FileID:            media.Meta.FileID,
		FileName:          media.Meta.FileName,
	})
	if err != nil {
		return fmt.Errorf("can not marshal download payload: %w", err)
	}
	return pub.PublishWork(ctx, pipeline.WorkSubject(domain.JobStepDownload), pipeline.WorkMsg{
		TaskID:  media.ID.Hex() + "-download",
		MediaID: media.ID.Hex(),
		Step:    string(domain.JobStepDownload),
		Attempt: 1,
		Payload: payload,
	})
}

func NewIngestDispatcher(jobs IJobService, pub pipeline.IPublisher, channelID, channelAccessHash int64) IIngestDispatcher {
	return &IngestDispatcher{
		jobs:              jobs,
		pub:               pub,
		channelID:         channelID,
		channelAccessHash: channelAccessHash,
	}
}
