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
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/amirdaaee/Glide/internals/workers"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

type Worker struct {
	wPool   worker.IWorkerPool
	byse    domain.IByseMediaRepository
	tempDir string
	ll      *zap.Logger
}

var _ workers.IStepHandler = (*Worker)(nil)

func (w *Worker) Handle(ctx context.Context, msg pipeline.WorkMsg) (*pipeline.ResultMsg, error) {
	ll := w.ll.Named("Handle").With(
		zap.String("task_id", msg.TaskID),
		zap.String("media_id", msg.MediaID),
		zap.Int("attempt", msg.Attempt),
	)
	res := &pipeline.ResultMsg{
		TaskID:  msg.TaskID,
		MediaID: msg.MediaID,
		Step:    msg.Step,
		Attempt: msg.Attempt,
	}
	var payload pipeline.DownloadPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		ll.Warn("invalid download payload", zap.Error(err))
		res.Error = &pipeline.StepError{Message: "invalid download payload", Permanent: true}
		return res, nil
	}
	ll.Info("downloading media",
		zap.Int64("channel_id", payload.ChannelID),
		zap.Int("message_id", payload.MessageID),
		zap.Int64("file_id", payload.FileID),
		zap.String("file_name", payload.FileName),
		zap.String("mime_type", payload.MimeType),
	)
	mediaID, err := bson.ObjectIDFromHex(msg.MediaID)
	if err != nil {
		ll.Warn("invalid media id", zap.Error(err))
		res.Error = &pipeline.StepError{Message: "invalid media id", Permanent: true}
		return res, nil
	}
	if payload.MessageID == 0 {
		ll.Warn("message id is required")
		res.Error = &pipeline.StepError{Message: "message id is required", Permanent: true}
		return res, nil
	}
	wrkr := w.wPool.GetNextWorker()
	if wrkr == nil {
		ll.Error("no available telegram worker")
		return nil, fmt.Errorf("no available telegram worker")
	}
	ll.Debug("fetching channel document", zap.Int("message_id", payload.MessageID))
	doc, err := wrkr.GetDoc(ctx, payload.MessageID)
	if err != nil {
		ll.Error("can not get channel document", zap.Error(err), zap.Int("message_id", payload.MessageID))
		return nil, fmt.Errorf("can not get channel document: %w", err)
	}
	ll.Info("downloading document", zap.Int64("doc_id", doc.ID), zap.Int64("size", doc.Size), zap.String("mime", doc.MimeType))
	tmpPath, err := w.downloadDocument(ctx, wrkr.API(), doc, mediaID)
	if err != nil {
		ll.Error("can not download document", zap.Error(err), zap.Int64("doc_id", doc.ID))
		return nil, err
	}
	ll.Debug("downloaded to temp file", zap.String("path", tmpPath))
	defer func() {
		if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
			ll.Warn("can not remove temp file", zap.String("path", tmpPath), zap.Error(rmErr))
		} else {
			ll.Debug("removed temp file", zap.String("path", tmpPath))
		}
	}()
	mime := payload.MimeType
	if mime == "" {
		mime = doc.MimeType
	}
	stored, size, err := w.uploadByse(ctx, tmpPath, payload.FileName, mime)
	if err != nil {
		ll.Error("can not upload to byse", zap.Error(err))
		return nil, err
	}
	ll.Info("media stored on byse",
		zap.String("file_code", stored.FileCode),
		zap.String("link", stored.Link),
		zap.Int64("size", size),
	)
	return downloadOK(res, stored, size)
}

func (w *Worker) downloadDocument(ctx context.Context, api *tg.Client, doc *tg.Document, mediaID bson.ObjectID) (string, error) {
	ll := w.ll.Named("downloadDocument").With(zap.String("media_id", mediaID.Hex()))
	if api == nil {
		ll.Error("telegram client is not ready")
		return "", fmt.Errorf("telegram client is not ready")
	}
	if doc == nil {
		ll.Error("channel document is nil")
		return "", fmt.Errorf("channel document is nil")
	}
	ll = ll.With(zap.Int64("doc_id", doc.ID), zap.Int64("size", doc.Size))
	dir := w.tempDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		ll.Error("can not create temp dir", zap.Error(err), zap.String("dir", dir))
		return "", fmt.Errorf("can not create temp dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, mediaID.Hex()+"-*.part")
	if err != nil {
		ll.Error("can not create temp file", zap.Error(err), zap.String("dir", dir))
		return "", fmt.Errorf("can not create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		ll.Error("can not close temp file", zap.Error(err), zap.String("path", tmpPath))
		return "", fmt.Errorf("can not close temp file: %w", err)
	}
	loc := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}
	ll.Debug("streaming document to disk", zap.String("path", tmpPath), zap.Int64("size", doc.Size))
	if _, err := downloader.NewDownloader().Download(api, loc).ToPath(ctx, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		ll.Error("can not download document", zap.Error(err), zap.String("path", tmpPath))
		return "", fmt.Errorf("can not download document: %w", err)
	}
	return tmpPath, nil
}

func (w *Worker) uploadByse(ctx context.Context, tmpPath, name, mimeType string) (*domain.ByseFile, int64, error) {
	ll := w.ll.Named("uploadByse").With(zap.String("path", tmpPath))
	body, err := os.Open(filepath.Clean(tmpPath))
	if err != nil {
		ll.Error("can not open temp file", zap.Error(err))
		return nil, 0, fmt.Errorf("can not open temp file: %w", err)
	}
	defer body.Close()
	stat, err := body.Stat()
	if err != nil {
		ll.Error("can not stat temp file", zap.Error(err))
		return nil, 0, fmt.Errorf("can not stat temp file: %w", err)
	}
	if name == "" {
		name = "file"
	}
	ll.Info("uploading file", zap.String("name", name), zap.Int64("size", stat.Size()), zap.String("mime", mimeType))
	created, err := w.byse.Create(ctx, name, body, stat.Size(), mimeType)
	if err != nil {
		ll.Error("can not upload byse file", zap.Error(err), zap.String("name", name))
		return nil, 0, fmt.Errorf("can not upload byse file: %w", err)
	}
	if created == nil || created.FileCode == "" {
		ll.Error("empty file code after upload")
		return nil, 0, fmt.Errorf("can not upload byse file: empty file code")
	}
	stored := created
	info, err := w.byse.Get(ctx, created.FileCode)
	if err != nil {
		ll.Warn("can not get byse file info after upload",
			zap.String("file_code", created.FileCode),
			zap.Error(err),
		)
	} else if info != nil {
		if info.FileCode == "" {
			info.FileCode = created.FileCode
		}
		stored = info
	}
	ll.Debug("byse upload complete", zap.String("file_code", stored.FileCode))
	return stored, stat.Size(), nil
}

func downloadOK(res *pipeline.ResultMsg, byse *domain.ByseFile, size int64) (*pipeline.ResultMsg, error) {
	raw, err := json.Marshal(pipeline.DownloadOutput{Byse: byse, Size: size})
	if err != nil {
		return nil, fmt.Errorf("can not marshal download output: %w", err)
	}
	res.OK = true
	res.Output = raw
	return res, nil
}

func New(wPool worker.IWorkerPool, byse domain.IByseMediaRepository, tempDir string) (*Worker, error) {
	if wPool == nil {
		return nil, fmt.Errorf("worker pool is nil")
	}
	if byse == nil {
		return nil, fmt.Errorf("byse media repository is nil")
	}
	ll := log.GetLogger(log.WORKERS).Named("download")
	ll.Info("download worker created", zap.String("temp_dir", tempDir))
	return &Worker{
		wPool:   wPool,
		byse:    byse,
		tempDir: tempDir,
		ll:      ll,
	}, nil
}
