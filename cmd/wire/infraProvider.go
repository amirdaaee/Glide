package wire

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository/byse"
	"github.com/amirdaaee/Glide/internals/repository/minio"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/amirdaaee/Glide/internals/workers/ingest"
	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ProvideMediaObjectRepository(cfg *config.ConfigType) (domain.IMediaObjectRepository, error) {
	return minio.NewMediaRepository(minio.Options{
		Endpoint:        cfg.MinioConfig.Endpoint,
		AccessKeyID:     cfg.MinioConfig.AccessKeyID,
		SecretAccessKey: cfg.MinioConfig.SecretAccessKey,
		Bucket:          cfg.MinioConfig.Bucket,
		UseSSL:          cfg.MinioConfig.UseSSL,
	})
}

func ProvideByseMediaRepository(cfg *config.ConfigType) (domain.IByseMediaRepository, error) {
	return byse.NewMediaRepository(byse.Options{
		BaseURL:       cfg.ByseConfig.BaseURL,
		APIKey:        cfg.ByseConfig.APIKey,
		Timeout:       cfg.ByseConfig.Timeout,
		UploadTimeout: cfg.ByseConfig.UploadTimeout,
	})
}

func ProvideIngestWorker(
	wPool worker.IWorkerPool,
	objects domain.IMediaObjectRepository,
	media service.IMediaService,
) (*ingest.Worker, error) {
	return ingest.New(wPool, objects, media)
}

func ProvideMongoDB(cfg *config.ConfigType) (*mongox.Database, error) {
	mCl, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoConfig.URI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongo: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), cfg.MongoConfig.PingTimeout)
	defer cancel()
	if err := mCl.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongo: %w", err)
	}
	cl := mongox.NewClient(mCl, &mongox.Config{})
	return cl.NewDatabase(cfg.MongoConfig.DB), nil
}
