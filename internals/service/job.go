package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// IJobService is the application API for media pipeline jobs.
type IJobService interface {
	// Create inserts a new media job.
	Create(ctx context.Context, job *domain.MediaJob) error
	// GetByMediaID returns the job for a media file.
	GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error)
	// GetByIdempotencyKey returns the job for an idempotency key.
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error)
	// Transition compare-and-sets the job step from from to to.
	Transition(ctx context.Context, mediaID bson.ObjectID, from, to domain.JobStep, ver int) (bool, error)
	// RecordFailure marks the job failed and records the error.
	RecordFailure(ctx context.Context, mediaID bson.ObjectID, step domain.JobStep, errMsg string) error
	// MarkTaskDone records a processed task and updates the job's last task ID.
	MarkTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string, step domain.JobStep) error
	// IsTaskDone reports whether taskID was already processed for mediaID.
	IsTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error)
}

// JobService implements IJobService.
type JobService struct {
	jobs domain.IJobRepository
}

var _ IJobService = (*JobService)(nil)

// nextJobStep is the legal forward transition for each pipeline step.
var nextJobStep = map[domain.JobStep]domain.JobStep{
	domain.JobStepIngest:   domain.JobStepDownload,
	domain.JobStepDownload: domain.JobStepDone,
	domain.JobStepNotify:   domain.JobStepDone,
}

// NextJobStep returns the legal next step after from.
func NextJobStep(from domain.JobStep) (domain.JobStep, bool) {
	to, ok := nextJobStep[from]
	return to, ok
}

// legalJobTransition reports whether from -> to is allowed.
func legalJobTransition(from, to domain.JobStep) bool {
	if to == domain.JobStepFailed {
		return from != domain.JobStepDone && from != domain.JobStepFailed
	}
	return nextJobStep[from] == to
}

// Create inserts a new media job.
func (s *JobService) Create(ctx context.Context, job *domain.MediaJob) error {
	if err := s.jobs.Create(ctx, job); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return err
		}
		return fmt.Errorf("can not create media job: %w", err)
	}
	return nil
}

// GetByMediaID returns the job for a media file.
func (s *JobService) GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error) {
	job, err := s.jobs.GetByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("can not get media job by media id: %w", err)
	}
	return job, nil
}

// GetByIdempotencyKey returns the job for an idempotency key.
func (s *JobService) GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error) {
	job, err := s.jobs.GetByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("can not get media job by idempotency key: %w", err)
	}
	return job, nil
}

// Transition compare-and-sets the job step from from to to.
func (s *JobService) Transition(ctx context.Context, mediaID bson.ObjectID, from, to domain.JobStep, ver int) (bool, error) {
	if !legalJobTransition(from, to) {
		return false, fmt.Errorf("job step %s -> %s: %w", from, to, domain.ErrInvalidTransition)
	}
	ok, err := s.jobs.CompareAndSetStep(ctx, mediaID, from, to, ver)
	if err != nil {
		return false, fmt.Errorf("can not transition media job: %w", err)
	}
	return ok, nil
}

// RecordFailure marks the job failed and records the error.
func (s *JobService) RecordFailure(ctx context.Context, mediaID bson.ObjectID, step domain.JobStep, errMsg string) error {
	job, err := s.jobs.GetByMediaID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("can not get media job by media id: %w", err)
	}
	if job.Attempts == nil {
		job.Attempts = map[string]int{}
	}
	job.Attempts[string(step)]++
	job.LastError = errMsg
	job.Step = domain.JobStepFailed
	if err := s.jobs.Save(ctx, job); err != nil {
		return fmt.Errorf("can not record media job failure: %w", err)
	}
	return nil
}

// MarkTaskDone records a processed task and updates the job's last task ID.
func (s *JobService) MarkTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string, step domain.JobStep) error {
	task := &domain.ProcessedTask{
		TaskID:  taskID,
		MediaID: mediaID,
		Step:    step,
		DoneAt:  time.Now().UTC(),
	}
	if err := s.jobs.CreateProcessedTask(ctx, task); err != nil && !errors.Is(err, domain.ErrAlreadyExists) {
		return fmt.Errorf("can not persist processed task: %w", err)
	}
	job, err := s.jobs.GetByMediaID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("can not get media job by media id: %w", err)
	}
	job.LastTaskID = taskID
	job.LastError = ""
	if err := s.jobs.Save(ctx, job); err != nil {
		return fmt.Errorf("can not mark task done: %w", err)
	}
	return nil
}

// IsTaskDone reports whether taskID was already processed for mediaID.
func (s *JobService) IsTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error) {
	done, err := s.jobs.HasProcessedTask(ctx, mediaID, taskID)
	if err != nil {
		return false, fmt.Errorf("can not check processed task: %w", err)
	}
	return done, nil
}

// NewJobService returns an IJobService.
func NewJobService(jobs domain.IJobRepository) IJobService {
	return &JobService{jobs: jobs}
}
