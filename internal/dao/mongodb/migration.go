package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"partivo_tickets/internal/conf"
)

// Migration defines the structure for a collection migration
type Migration struct {
	Collection string
	Indexes    []mongo.IndexModel
}

// collection相關的index
var migrations = []Migration{
	// {
	//   Collection: "products",
	//   Indexes: []mongo.IndexModel{
	//     {
	//       Keys:    bson.D{{Key: "sku", Value: 1}},
	//       Options: options.Index().SetUnique(true),
	//     },
	//   },
	// },
}

// Migrate runs all the defined migrations.
func Migrate(client *mongo.Client, cfg *conf.MongodbConfig) error {
	log.Println("Running MongoDB migrations...")
	db := client.Database(cfg.DB)

	for _, m := range migrations {
		coll := db.Collection(m.Collection)

		if len(m.Indexes) > 0 {
			_, err := coll.Indexes().CreateMany(context.Background(), m.Indexes)
			if err != nil {
				return fmt.Errorf("failed to create indexes for collection '%s': %w", m.Collection, err)
			}
		}
	}

	return nil
}
