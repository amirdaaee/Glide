package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

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

// Worker extracts a thumbnail from a channel document and stores it.
type Worker struct {
	wPool   worker.IWorkerPool
	objects domain.IMediaObjectRepository
	ll      *zap.Logger
}

var _ workers.IStepHandler = (*Worker)(nil)

// Handle downloads the document thumbnail and stores it as an object.
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
	var payload pipeline.IngestPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		ll.Warn("invalid ingest payload", zap.Error(err))
		res.Error = &pipeline.StepError{Message: "invalid ingest payload", Permanent: true}
		return res, nil
	}
	ll.Info("ingesting media",
		zap.Int64("channel_id", payload.ChannelID),
		zap.Int("message_id", payload.MessageID),
		zap.Int64("file_id", payload.FileID),
	)
	mediaID, err := bson.ObjectIDFromHex(msg.MediaID)
	if err != nil {
		ll.Warn("invalid media id", zap.Error(err))
		res.Error = &pipeline.StepError{Message: "invalid media id", Permanent: true}
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
	ll.Debug("downloading thumbnail", zap.Int64("doc_id", doc.ID), zap.Int64("size", doc.Size))
	thumb, contentType, err := downloadThumbnail(ctx, wrkr.API(), doc)
	if err != nil {
		ll.Warn("can not download thumbnail", zap.Error(err), zap.Int64("doc_id", doc.ID))
		res.Error = &pipeline.StepError{Message: err.Error(), Permanent: true}
		return res, nil
	}
	ll.Debug("storing thumbnail", zap.Int("bytes", len(thumb)), zap.String("content_type", contentType))
	url, err := w.objects.PutThumbnail(ctx, mediaID, bytes.NewReader(thumb), int64(len(thumb)), contentType)
	if err != nil {
		ll.Error("can not store thumbnail", zap.Error(err))
		return nil, fmt.Errorf("can not store thumbnail: %w", err)
	}
	out, err := json.Marshal(pipeline.IngestOutput{ThumbnailURL: url})
	if err != nil {
		ll.Error("can not marshal ingest output", zap.Error(err))
		return nil, fmt.Errorf("can not marshal ingest output: %w", err)
	}
	res.OK = true
	res.Output = out
	ll.Info("thumbnail stored", zap.String("url", url), zap.Int("bytes", len(thumb)))
	return res, nil
}

// downloadThumbnail returns JPEG/PNG/WebP thumbnail bytes for doc.
func downloadThumbnail(ctx context.Context, api *tg.Client, doc *tg.Document) ([]byte, string, error) {
	if api == nil {
		return nil, "", fmt.Errorf("telegram client is not ready")
	}
	thumbType, cached := pickThumbnail(doc.Thumbs)
	if len(cached) > 0 {
		return cached, "image/jpeg", nil
	}
	if thumbType == "" {
		return nil, "", fmt.Errorf("document has no thumbnail")
	}
	loc := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
		ThumbSize:     thumbType,
	}
	var buf bytes.Buffer
	kind, err := downloader.NewDownloader().Download(api, loc).Stream(ctx, &buf)
	if err != nil {
		return nil, "", fmt.Errorf("can not download thumbnail: %w", err)
	}
	return buf.Bytes(), mimeFromStorage(kind), nil
}

// pickThumbnail selects the largest thumbnail, preferring cached bytes.
func pickThumbnail(thumbs []tg.PhotoSizeClass) (thumbType string, cached []byte) {
	bestArea := -1
	for _, t := range thumbs {
		switch s := t.(type) {
		case *tg.PhotoSize:
			if area := s.W * s.H; area > bestArea {
				bestArea = area
				thumbType = s.Type
				cached = nil
			}
		case *tg.PhotoSizeProgressive:
			if area := s.W * s.H; area > bestArea {
				bestArea = area
				thumbType = s.Type
				cached = nil
			}
		case *tg.PhotoCachedSize:
			if area := s.W * s.H; area > bestArea {
				bestArea = area
				thumbType = ""
				cached = s.Bytes
			}
		}
	}
	return thumbType, cached
}

// mimeFromStorage maps a Telegram storage file type to a MIME type.
func mimeFromStorage(kind tg.StorageFileTypeClass) string {
	switch kind.(type) {
	case *tg.StorageFilePng:
		return "image/png"
	case *tg.StorageFileWebp:
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// New returns an ingest step worker.
func New(wPool worker.IWorkerPool, objects domain.IMediaObjectRepository) (*Worker, error) {
	if wPool == nil {
		return nil, fmt.Errorf("worker pool is nil")
	}
	if objects == nil {
		return nil, fmt.Errorf("media object repository is nil")
	}
	ll := log.GetLogger(log.WORKERS).Named("ingest")
	ll.Info("ingest worker created")
	return &Worker{
		wPool:   wPool,
		objects: objects,
		ll:      ll,
	}, nil
}
