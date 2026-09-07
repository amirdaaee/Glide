package minio

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/amirdaaee/Glide/internals/domain"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Options struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

type MediaRepository struct {
	client   *miniogo.Client
	bucket   string
	endpoint string
	useSSL   bool
}

var _ domain.IMediaObjectRepository = (*MediaRepository)(nil)

func (r *MediaRepository) PutThumbnail(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error) {
	key := fmt.Sprintf("thumbs/%s%s", id.Hex(), extFromContentType(contentType))
	return r.put(ctx, key, body, size, contentType)
}

func (r *MediaRepository) Put(ctx context.Context, id bson.ObjectID, body io.Reader, size int64, contentType string) (string, error) {
	key := fmt.Sprintf("media/%s%s", id.Hex(), extFromContentType(contentType))
	return r.put(ctx, key, body, size, contentType)
}

func (r *MediaRepository) put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (string, error) {
	opts := miniogo.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	if _, err := r.client.PutObject(ctx, r.bucket, key, body, size, opts); err != nil {
		return "", fmt.Errorf("can not put object %s: %w", key, err)
	}
	return fmt.Sprintf("/%s/%s", r.bucket, key), nil
}

func extFromContentType(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	default:
		if strings.HasPrefix(strings.ToLower(contentType), "video/") {
			return ".mp4"
		}
		return ".jpg"
	}
}

func NewMediaRepository(opts Options) (domain.IMediaObjectRepository, error) {
	cl, err := miniogo.New(opts.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKeyID, opts.SecretAccessKey, ""),
		Secure: opts.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("can not create minio client: %w", err)
	}
	ctx := context.Background()
	exists, err := cl.BucketExists(ctx, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("can not check minio bucket: %w", err)
	}
	if !exists {
		if err := cl.MakeBucket(ctx, opts.Bucket, miniogo.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("can not create minio bucket %s: %w", opts.Bucket, err)
		}
	}
	return &MediaRepository{
		client:   cl,
		bucket:   opts.Bucket,
		endpoint: opts.Endpoint,
		useSSL:   opts.UseSSL,
	}, nil
}
