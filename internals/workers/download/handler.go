package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/amirdaaee/Glide/internals/workers"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

type Worker struct {
	wPool   worker.IWorkerPool
	media   service.IMediaService
	byse    domain.IByseMediaRepository
	tempDir string
	ll      *zap.Logger
}

var _ workers.IStepHandler = (*Worker)(nil)

func (w *Worker) Handle(ctx context.Context, msg pipeline.WorkMsg) (*pipeline.ResultMsg, error) {
	ll := w.ll.Named("Handle").With(
		zap.String("task_id", msg.TaskID),
		zap.String("media_id", msg.MediaID),
	)
	res := &pipeline.ResultMsg{
		TaskID:  msg.TaskID,
		MediaID: msg.MediaID,
		Step:    msg.Step,
		Attempt: msg.Attempt,
	}
	var payload pipeline.DownloadPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		res.Error = &pipeline.StepError{Message: "invalid download payload", Permanent: true}
		return res, nil
	}
	mediaID, err := bson.ObjectIDFromHex(msg.MediaID)
	if err != nil {
		res.Error = &pipeline.StepError{Message: "invalid media id", Permanent: true}
		return res, nil
	}
	media, err := w.media.Get(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("can not get media file: %w", err)
	}
	if media.HasByse() {
		return downloadOK(res, media.Byse, media.Meta.FileSize)
	}
	messageID := payload.MessageID
	if messageID == 0 {
		messageID = media.MessageID
	}
	if messageID == 0 {
		res.Error = &pipeline.StepError{Message: "message id is required", Permanent: true}
		return res, nil
	}
	wrkr := w.wPool.GetNextWorker()
	if wrkr == nil {
		return nil, fmt.Errorf("no available telegram worker")
	}
	doc, err := wrkr.GetDoc(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("can not get channel document: %w", err)
	}
	tmpPath, err := w.downloadDocument(ctx, wrkr.API(), doc, mediaID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
			ll.Warn("can not remove temp file", zap.String("path", tmpPath), zap.Error(rmErr))
		}
	}()
	stored, size, err := w.uploadByse(ctx, tmpPath, media, payload.FileName)
	if err != nil {
		return nil, err
	}
	if err := w.media.SetStored(ctx, mediaID, stored); err != nil {
		return nil, fmt.Errorf("can not persist byse file: %w", err)
	}
	ll.Info("media stored on byse",
		zap.String("file_code", stored.FileCode),
		zap.String("link", stored.Link),
		zap.Int64("size", size),
	)
	return downloadOK(res, stored, size)
}

func (w *Worker) downloadDocument(ctx context.Context, api *tg.Client, doc *tg.Document, mediaID bson.ObjectID) (string, error) {
	if api == nil {
		return "", fmt.Errorf("telegram client is not ready")
	}
	if doc == nil {
		return "", fmt.Errorf("channel document is nil")
	}
	dir := w.tempDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("can not create temp dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, mediaID.Hex()+"-*.part")
	if err != nil {
		return "", fmt.Errorf("can not create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("can not close temp file: %w", err)
	}
	loc := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}
	if _, err := downloader.NewDownloader().Download(api, loc).ToPath(ctx, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("can not download document: %w", err)
	}
	return tmpPath, nil
}

func (w *Worker) uploadByse(ctx context.Context, tmpPath string, media *domain.MediaFile, payloadName string) (*domain.ByseFile, int64, error) {
	body, err := os.Open(filepath.Clean(tmpPath))
	if err != nil {
		return nil, 0, fmt.Errorf("can not open temp file: %w", err)
	}
	defer body.Close()
	stat, err := body.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("can not stat temp file: %w", err)
	}
	name := media.Meta.FileName
	if name == "" {
		name = payloadName
	}
	if name == "" {
		name = "file"
	}
	created, err := w.byse.Create(ctx, name, body, stat.Size(), media.Meta.MimeType)
	if err != nil {
		return nil, 0, fmt.Errorf("can not upload byse file: %w", err)
	}
	if created == nil || created.FileCode == "" {
		return nil, 0, fmt.Errorf("can not upload byse file: empty file code")
	}
	stored := created
	info, err := w.byse.Get(ctx, created.FileCode)
	if err != nil {
		w.ll.Named("uploadByse").Warn("can not get byse file info after upload",
			zap.String("file_code", created.FileCode),
			zap.Error(err),
		)
	} else if info != nil {
		if info.FileCode == "" {
			info.FileCode = created.FileCode
		}
		stored = info
	}
	return stored, stat.Size(), nil
}

func downloadOK(res *pipeline.ResultMsg, byse *domain.ByseFile, size int64) (*pipeline.ResultMsg, error) {
	out := pipeline.DownloadOutput{Size: size}
	if byse != nil {
		out.FileCode = byse.FileCode
		out.Link = byse.Link
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("can not marshal download output: %w", err)
	}
	res.OK = true
	res.Output = raw
	return res, nil
}

func New(wPool worker.IWorkerPool, media service.IMediaService, byse domain.IByseMediaRepository, tempDir string) (*Worker, error) {
	if wPool == nil {
		return nil, fmt.Errorf("worker pool is nil")
	}
	if media == nil {
		return nil, fmt.Errorf("media service is nil")
	}
	if byse == nil {
		return nil, fmt.Errorf("byse media repository is nil")
	}
	return &Worker{
		wPool:   wPool,
		media:   media,
		byse:    byse,
		tempDir: tempDir,
		ll:      log.GetLogger(log.WORKERS).Named("download"),
	}, nil
}
