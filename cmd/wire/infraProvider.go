package wire

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/config"
	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

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
