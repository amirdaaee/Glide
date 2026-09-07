package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

type IIngestDispatcher interface {
	Dispatch(ctx context.Context, media *domain.MediaFile) error
}

type IngestDispatcher struct {
	jobs      IJobService
	pub       pipeline.IPublisher
	channelID int64
	ll        *zap.Logger
}

var _ IIngestDispatcher = (*IngestDispatcher)(nil)

func (d *IngestDispatcher) Dispatch(ctx context.Context, media *domain.MediaFile) error {
	ll := d.ll.Named("Dispatch")
	if media == nil || media.ID.IsZero() {
		ll.Error("media id is required")
		return fmt.Errorf("media id is required")
	}
	ll = ll.With(
		zap.String("media_id", media.ID.Hex()),
		zap.Int64("file_id", media.Meta.FileID),
		zap.Int("message_id", media.MessageID),
	)
	if media.HasByse() {
		ll.Info("media already stored, skipping pipeline")
		return nil
	}
	if media.ThumbnailURL != "" {
		ll.Info("thumbnail already present, dispatching download")
		return d.dispatchDownload(ctx, media)
	}
	job, err := d.jobs.GetByMediaID(ctx, media.ID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			ll.Error("can not get media job", zap.Error(err))
			return err
		}
		ll.Info("creating ingest job")
		job = &domain.MediaJob{
			MediaID:        media.ID,
			Step:           domain.JobStepIngest,
			StatusVer:      0,
			IdempotencyKey: media.ID.Hex(),
		}
		if err := d.jobs.Create(ctx, job); err != nil {
			if !errors.Is(err, domain.ErrAlreadyExists) {
				ll.Error("can not create ingest job", zap.Error(err))
				return err
			}
			ll.Info("ingest job already exists")
		}
	} else if job.Step != domain.JobStepIngest {
		ll.Info("job is not in ingest step, skipping", zap.String("step", string(job.Step)))
		return nil
	}
	payload, err := json.Marshal(pipeline.IngestPayload{
		ChannelID: d.channelID,
		MessageID: media.MessageID,
		FileID:    media.Meta.FileID,
	})
	if err != nil {
		ll.Error("can not marshal ingest payload", zap.Error(err))
		return fmt.Errorf("can not marshal ingest payload: %w", err)
	}
	taskID := media.ID.Hex() + "-ingest"
	if err := d.pub.PublishWork(ctx, pipeline.WorkSubject(domain.JobStepIngest), pipeline.WorkMsg{
		TaskID:  taskID,
		MediaID: media.ID.Hex(),
		Step:    string(domain.JobStepIngest),
		Attempt: 1,
		Payload: payload,
	}); err != nil {
		ll.Error("can not publish ingest work", zap.Error(err), zap.String("task_id", taskID))
		return err
	}
	ll.Info("published ingest work", zap.String("task_id", taskID))
	return nil
}

func (d *IngestDispatcher) dispatchDownload(ctx context.Context, media *domain.MediaFile) error {
	ll := d.ll.Named("dispatchDownload").With(zap.String("media_id", media.ID.Hex()))
	job, err := d.jobs.GetByMediaID(ctx, media.ID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			ll.Error("can not get media job", zap.Error(err))
			return err
		}
		ll.Info("creating download job")
		job = &domain.MediaJob{
			MediaID:        media.ID,
			Step:           domain.JobStepDownload,
			StatusVer:      0,
			IdempotencyKey: media.ID.Hex(),
		}
		if err := d.jobs.Create(ctx, job); err != nil {
			if !errors.Is(err, domain.ErrAlreadyExists) {
				ll.Error("can not create download job", zap.Error(err))
				return err
			}
			ll.Info("download job already exists")
		}
	} else if job.Step == domain.JobStepIngest {
		ll.Info("transitioning ingest job to download")
		if _, err := d.jobs.Transition(ctx, media.ID, domain.JobStepIngest, domain.JobStepDownload, job.StatusVer); err != nil {
			ll.Error("can not transition job to download", zap.Error(err))
			return err
		}
	} else if job.Step != domain.JobStepDownload {
		ll.Info("job is not in download step, skipping", zap.String("step", string(job.Step)))
		return nil
	}
	if err := PublishDownloadWork(ctx, d.pub, media, d.channelID); err != nil {
		ll.Error("can not publish download work", zap.Error(err))
		return err
	}
	ll.Info("published download work")
	return nil
}

func PublishDownloadWork(ctx context.Context, pub pipeline.IPublisher, media *domain.MediaFile, channelID int64) error {
	if pub == nil {
		return fmt.Errorf("publisher is nil")
	}
	if media == nil || media.ID.IsZero() {
		return fmt.Errorf("media id is required")
	}
	payload, err := json.Marshal(pipeline.DownloadPayload{
		ChannelID: channelID,
		MessageID: media.MessageID,
		FileID:    media.Meta.FileID,
		FileName:  media.Meta.FileName,
		MimeType:  media.Meta.MimeType,
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

func PublishNotifyWork(ctx context.Context, pub pipeline.IPublisher, mediaID bson.ObjectID, status domain.MediaStatus) error {
	if pub == nil {
		return fmt.Errorf("publisher is nil")
	}
	if mediaID.IsZero() {
		return fmt.Errorf("media id is required")
	}
	payload, err := json.Marshal(pipeline.NotifyPayload{Status: status})
	if err != nil {
		return fmt.Errorf("can not marshal notify payload: %w", err)
	}
	return pub.PublishWork(ctx, pipeline.WorkSubject(domain.JobStepNotify), pipeline.WorkMsg{
		TaskID:  mediaID.Hex() + "-notify-" + string(status),
		MediaID: mediaID.Hex(),
		Step:    string(domain.JobStepNotify),
		Attempt: 1,
		Payload: payload,
	})
}

func NewIngestDispatcher(jobs IJobService, pub pipeline.IPublisher, channelID int64) IIngestDispatcher {
	return &IngestDispatcher{
		jobs:      jobs,
		pub:       pub,
		channelID: channelID,
		ll:        log.GetLogger(log.PIPELINE).Named("ingest"),
	}
}
