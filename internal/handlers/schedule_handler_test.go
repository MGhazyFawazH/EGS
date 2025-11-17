package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"

	"github.com/mghazyfawazh/EGS/internal/handlers"
	"github.com/mghazyfawazh/EGS/internal/models"
	"github.com/mghazyfawazh/EGS/internal/repo"
)

func setupTest() (*gin.Engine, *repo.MongoRepo, *handlers.Handler) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	db := client.Database("testdb")
	coll := db.Collection("schedules")

	repo := repo.NewMongoRepo(coll)
	h := handlers.NewHandler(repo, coll)

	gin.SetMode(gin.TestMode)
	r := gin.Default()

	r.POST("/create", h.Create)
	r.GET("/get/:uuid", h.GetByUUID)
	r.DELETE("/delete/:uuid", h.Delete)
	r.GET("/teacher", h.TeacherSchedule)
	r.GET("/student", h.StudentSchedule)

	return r, repo, h
}

func TestCreateSuccess(t *testing.T) {
	r, _, _ := setupTest()

	body := `{
		"class_code": "XTKJ1",
		"class_name": "TKJ Dasar",
		"subject_code": "TKJ01",
		"teacher_nik": "12345",
		"teacher_name": "Budi",
		"date": "2024-01-01",
		"jam_ke": 1,
		"time_start": "07:00:00",
		"time_end": "08:00:00"
	}`

	req, _ := http.NewRequest("POST", "/create", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	assert.Equal(t, 201, resp.Code)
}

func TestCreateConflict(t *testing.T) {
	r, _, h := setupTest()

	now := time.Now()
	s := &models.Schedule{
		UUID:        uuid.New().String(),
		ClassCode:   "XTKJ1",
		ClassName:   "TKJ Dasar",
		SubjectCode: "TKJ01",
		TeacherNIK:  "12345",
		TeacherName: "Budi",
		Date:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		JamKe:       1,
		TimeStart:   "07:00:00",
		TimeEnd:     "08:00:00",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_ = h.Repo.Insert(s)

	body := `{
		"class_code": "XTKJ1",
		"class_name": "TKJ Dasar",
		"subject_code": "TKJ01",
		"teacher_nik": "12345",
		"teacher_name": "Budi",
		"date": "2024-01-01",
		"jam_ke": 1,
		"time_start": "07:30:00",
		"time_end": "08:30:00"
	}`

	req, _ := http.NewRequest("POST", "/create", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	assert.Equal(t, 409, resp.Code)
}

func TestGetByUUID(t *testing.T) {
	r, repo, _ := setupTest()

	now := time.Now()
	id := uuid.New().String()

	s := &models.Schedule{
		UUID:        id,
		ClassCode:   "XTKJ1",
		ClassName:   "TKJ Dasar",
		SubjectCode: "TKJ01",
		TeacherNIK:  "999",
		TeacherName: "Test",
		Date:        now,
		JamKe:       1,
		TimeStart:   "07:00:00",
		TimeEnd:     "08:00:00",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_ = repo.Insert(s)

	req, _ := http.NewRequest("GET", "/get/"+id, nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, 200, resp.Code)

	var out models.Schedule
	json.Unmarshal(resp.Body.Bytes(), &out)
	assert.Equal(t, id, out.UUID)
}

func TestDelete(t *testing.T) {
	r, repo, _ := setupTest()

	now := time.Now()
	id := uuid.New().String()

	s := &models.Schedule{
		UUID:        id,
		ClassCode:   "XTKJ1",
		ClassName:   "TKJ Dasar",
		SubjectCode: "TKJ01",
		TeacherNIK:  "999",
		TeacherName: "Test",
		Date:        now,
		JamKe:       1,
		TimeStart:   "07:00:00",
		TimeEnd:     "08:00:00",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_ = repo.Insert(s)

	req, _ := http.NewRequest("DELETE", "/delete/"+id, nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, 200, resp.Code)
}
