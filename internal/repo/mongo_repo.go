package repo

import (
	"context"
	"errors"

	"github.com/mghazyfawazh/EGS/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepo struct {
	Coll *mongo.Collection
	Ctx  context.Context
}

func NewMongoRepo(coll *mongo.Collection) *MongoRepo {
	ctx := context.Background()

	// Index by date + class_code
	_, _ = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "date", Value: 1},
			{Key: "class_code", Value: 1},
		},
		Options: options.Index().SetBackground(true),
	})

	_, _ = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "date", Value: 1},
			{Key: "teacher_nik", Value: 1},
		},
		Options: options.Index().SetBackground(true),
	})

	return &MongoRepo{Coll: coll, Ctx: ctx}
}

func (r *MongoRepo) Insert(s *models.Schedule) error {
	_, err := r.Coll.InsertOne(r.Ctx, s)
	return err
}

func (r *MongoRepo) FindAll() ([]models.Schedule, error) {
	cur, err := r.Coll.Find(r.Ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(r.Ctx)

	var out []models.Schedule
	if err := cur.All(r.Ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MongoRepo) FindByUUID(uuid string) (*models.Schedule, error) {
	var s models.Schedule
	if err := r.Coll.FindOne(r.Ctx, bson.M{"uuid": uuid}).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *MongoRepo) UpdateByUUID(uuid string, update bson.M) error {
	res, err := r.Coll.UpdateOne(r.Ctx, bson.M{"uuid": uuid}, bson.M{"$set": update})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("not found")
	}
	return nil
}

func (r *MongoRepo) DeleteByUUID(uuid string) error {
	res, err := r.Coll.DeleteOne(r.Ctx, bson.M{"uuid": uuid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("not found")
	}
	return nil
}
