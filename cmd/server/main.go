package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	_ "github.com/mghazyfawazh/EGS/docs"

	"github.com/mghazyfawazh/EGS/internal/handlers"
	"github.com/mghazyfawazh/EGS/internal/middleware"
	"github.com/mghazyfawazh/EGS/internal/repo"

	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
)

func connectMongo(uri string) *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("mongo ping failed:", err)
	}

	return client
}

func main() {
	godotenv.Load()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI env required")
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "school"
	}

	client := connectMongo(mongoURI)
	db := client.Database(dbName)
	coll := db.Collection("schedules")

	mrepo := repo.NewMongoRepo(coll)
	h := handlers.NewHandler(mrepo, coll)

	r := gin.Default()

	// Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	api := r.Group("/api", middleware.APIKeyAuth())
	{
		s := api.Group("/schedules")
		{
			s.POST("", h.Create)
			s.GET("", h.GetAll)
			s.GET("/:uuid", h.GetByUUID)
			s.PUT("/:uuid", h.Update)
			s.DELETE("/:uuid", h.Delete)
			s.GET("/student", h.StudentSchedule)
			s.GET("/teacher", h.TeacherSchedule)
			s.GET("/export", h.ExportJP)
			s.POST("/import", h.ImportExcel)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("listening on", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

