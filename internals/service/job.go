package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IJobService interface {
	Create(ctx context.Context, job *domain.MediaJob) error
	GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error)
	Transition(ctx context.Context, mediaID bson.ObjectID, from, to domain.JobStep, ver int) (bool, error)
	RecordFailure(ctx context.Context, mediaID bson.ObjectID, step domain.JobStep, errMsg string) error
	MarkTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string, step domain.JobStep) error
	IsTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error)
}

type JobService struct {
	jobs domain.IJobRepository
}

var _ IJobService = (*JobService)(nil)

var nextJobStep = map[domain.JobStep]domain.JobStep{
	domain.JobStepIngest:   domain.JobStepDownload,
	domain.JobStepDownload: domain.JobStepDone,
	domain.JobStepNotify:   domain.JobStepDone,
}

func NextJobStep(from domain.JobStep) (domain.JobStep, bool) {
	to, ok := nextJobStep[from]
	return to, ok
}

func legalJobTransition(from, to domain.JobStep) bool {
	if to == domain.JobStepFailed {
		return from != domain.JobStepDone && from != domain.JobStepFailed
	}
	return nextJobStep[from] == to
}

func (s *JobService) Create(ctx context.Context, job *domain.MediaJob) error {
	if err := s.jobs.Create(ctx, job); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return err
		}
		return fmt.Errorf("can not create media job: %w", err)
	}
	return nil
}

func (s *JobService) GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error) {
	job, err := s.jobs.GetByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("can not get media job by media id: %w", err)
	}
	return job, nil
}

func (s *JobService) GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error) {
	job, err := s.jobs.GetByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("can not get media job by idempotency key: %w", err)
	}
	return job, nil
}

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

func (s *JobService) IsTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error) {
	done, err := s.jobs.HasProcessedTask(ctx, mediaID, taskID)
	if err != nil {
		return false, fmt.Errorf("can not check processed task: %w", err)
	}
	return done, nil
}

func NewJobService(jobs domain.IJobRepository) IJobService {
	return &JobService{jobs: jobs}
}
