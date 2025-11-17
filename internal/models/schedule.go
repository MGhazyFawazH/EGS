package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Schedule struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UUID        string             `bson:"uuid" json:"uuid"`
	ClassCode   string             `bson:"class_code" json:"class_code"`
	ClassName   string             `bson:"class_name" json:"class_name"`
	SubjectCode string             `bson:"subject_code" json:"subject_code"`
	TeacherNIK  string             `bson:"teacher_nik" json:"teacher_nik"`
	TeacherName string             `bson:"teacher_name" json:"teacher_name"`
	Date        time.Time          `bson:"date" json:"date"`          
	JamKe       int                `bson:"jam_ke" json:"jam_ke"`       
	TimeStart   string             `bson:"time_start" json:"time_start"` 
	TimeEnd     string             `bson:"time_end" json:"time_end"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
