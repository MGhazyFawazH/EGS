package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mghazyfawazh/EGS/internal/models"
	"github.com/mghazyfawazh/EGS/internal/repo"
)

type Handler struct {
	Repo *repo.MongoRepo
	Coll *mongo.Collection
	Ctx  context.Context
}

func NewHandler(r *repo.MongoRepo, coll *mongo.Collection) *Handler {
	return &Handler{Repo: r, Coll: coll, Ctx: context.Background()}
}

func BsonE(key string, value interface{}) primitive.E {
	return primitive.E{Key: key, Value: value}
}

func authorize(c *gin.Context) bool {
	expected := os.Getenv("API_KEY")
	if expected == "" {
		expected = "SECRET123"
	}
	got := c.GetHeader("x-api-key")
	if got == "" || got != expected {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}
	return true
}

func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

func timeOverlap(s1, e1, s2, e2 string) (bool, error) {
	t1s, err := time.Parse("15:04:05", s1)
	if err != nil {
		return false, err
	}
	t1e, err := time.Parse("15:04:05", e1)
	if err != nil {
		return false, err
	}
	t2s, err := time.Parse("15:04:05", s2)
	if err != nil {
		return false, err
	}
	t2e, err := time.Parse("15:04:05", e2)
	if err != nil {
		return false, err
	}

	if !(t1e.Before(t2s) || t1e.Equal(t2s) || t2e.Before(t1s) || t2e.Equal(t1s)) {
		return true, nil
	}

	if t1e.Equal(t2s) || t2e.Equal(t1s) {
		return true, nil
	}

	return false, nil
}

func (h *Handler) DetectConflict(date time.Time, classCode, teacherNIK, timeStart, timeEnd, excludeUUID string) (bool, error) {
	filter := bson.M{"date": date}
	cur, err := h.Coll.Find(h.Ctx, filter)
	if err != nil {
		return false, err
	}
	var rows []models.Schedule
	if err := cur.All(h.Ctx, &rows); err != nil {
		return false, err
	}
	for _, s := range rows {
		if excludeUUID != "" && s.UUID == excludeUUID {
			continue
		}
		if s.ClassCode == classCode || s.TeacherNIK == teacherNIK {
			ok, err := timeOverlap(s.TimeStart, s.TimeEnd, timeStart, timeEnd)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func (h *Handler) Create(c *gin.Context) {
	if !authorize(c) {
		return
	}
	var in struct {
		ClassCode   string `json:"class_code" binding:"required"`
		ClassName   string `json:"class_name" binding:"required"`
		SubjectCode string `json:"subject_code" binding:"required"`
		TeacherNIK  string `json:"teacher_nik" binding:"required"`
		TeacherName string `json:"teacher_name" binding:"required"`
		Date        string `json:"date" binding:"required"` 
		JamKe       int    `json:"jam_ke" binding:"required"`
		TimeStart   string `json:"time_start" binding:"required"` 
		TimeEnd     string `json:"time_end" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	date, err := parseDate(in.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format (expected YYYY-MM-DD)"})
		return
	}

	conflict, err := h.DetectConflict(date, in.ClassCode, in.TeacherNIK, in.TimeStart, in.TimeEnd, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conflict {
		c.JSON(http.StatusConflict, gin.H{"error": "schedule conflict detected"})
		return
	}
	now := time.Now()
	s := &models.Schedule{
		UUID:        uuid.New().String(),
		ClassCode:   in.ClassCode,
		ClassName:   in.ClassName,
		SubjectCode: in.SubjectCode,
		TeacherNIK:  in.TeacherNIK,
		TeacherName: in.TeacherName,
		Date:        date,
		JamKe:       in.JamKe,
		TimeStart:   in.TimeStart,
		TimeEnd:     in.TimeEnd,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.Repo.Insert(s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h *Handler) GetAll(c *gin.Context) {
	if !authorize(c) {
		return
	}
	rows, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) GetByUUID(c *gin.Context) {
	if !authorize(c) {
		return
	}
	uuidStr := c.Param("uuid")
	if uuidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid required"})
		return
	}
	row, err := h.Repo.FindByUUID(uuidStr)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *Handler) Update(c *gin.Context) {
	if !authorize(c) {
		return
	}
	uuidParam := c.Param("uuid")
	if uuidParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid required"})
		return
	}
	var in struct {
		ClassCode   string `json:"class_code"`
		ClassName   string `json:"class_name"`
		SubjectCode string `json:"subject_code"`
		TeacherNIK  string `json:"teacher_nik"`
		TeacherName string `json:"teacher_name"`
		Date        string `json:"date"`
		JamKe       *int   `json:"jam_ke"`
		TimeStart   string `json:"time_start"`
		TimeEnd     string `json:"time_end"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	existing, err := h.Repo.FindByUUID(uuidParam)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	date := existing.Date
	if in.Date != "" {
		if d, err := parseDate(in.Date); err == nil {
			date = d
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format"})
			return
		}
	}
	classCode := existing.ClassCode
	if in.ClassCode != "" {
		classCode = in.ClassCode
	}
	teacherNIK := existing.TeacherNIK
	if in.TeacherNIK != "" {
		teacherNIK = in.TeacherNIK
	}
	timeStart := existing.TimeStart
	if in.TimeStart != "" {
		timeStart = in.TimeStart
	}
	timeEnd := existing.TimeEnd
	if in.TimeEnd != "" {
		timeEnd = in.TimeEnd
	}

	conflict, err := h.DetectConflict(date, classCode, teacherNIK, timeStart, timeEnd, uuidParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conflict {
		c.JSON(http.StatusConflict, gin.H{"error": "schedule conflict detected"})
		return
	}
	update := bson.M{
		"updated_at":  time.Now(),
		"date":        date,
		"class_code":  classCode,
		"time_start":  timeStart,
		"time_end":    timeEnd,
		"teacher_nik": teacherNIK,
	}
	if in.ClassName != "" {
		update["class_name"] = in.ClassName
	}
	if in.SubjectCode != "" {
		update["subject_code"] = in.SubjectCode
	}
	if in.TeacherName != "" {
		update["teacher_name"] = in.TeacherName
	}
	if in.JamKe != nil {
		update["jam_ke"] = *in.JamKe
	}
	if err := h.Repo.UpdateByUUID(uuidParam, update); err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) Delete(c *gin.Context) {
	if !authorize(c) {
		return
	}
	uuidStr := c.Param("uuid")
	if uuidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid required"})
		return
	}
	if err := h.Repo.DeleteByUUID(uuidStr); err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) StudentSchedule(c *gin.Context) {
	if !authorize(c) {
		return
	}
	classCode := c.Query("class_code")
	dateStr := c.Query("date")
	if classCode == "" || dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "class_code and date required"})
		return
	}
	date, err := parseDate(dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	cur, err := h.Coll.Find(h.Ctx, bson.M{"class_code": classCode, "date": date})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var out []models.Schedule
	if err := cur.All(h.Ctx, &out); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) TeacherSchedule(c *gin.Context) {
	if !authorize(c) {
		return
	}
	nik := c.Query("teacher_nik")
	sd := c.Query("start_date")
	ed := c.Query("end_date")
	if nik == "" || sd == "" || ed == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "teacher_nik, start_date, end_date required"})
		return
	}
	start, err := parseDate(sd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date"})
		return
	}
	end, err := parseDate(ed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date"})
		return
	}
	filter := bson.M{
		"teacher_nik": nik,
		"date":        bson.M{"$gte": start, "$lte": end},
	}
	cur, err := h.Coll.Find(h.Ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var rows []models.Schedule
	if err := cur.All(h.Ctx, &rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totalJP := len(rows)
	c.JSON(http.StatusOK, gin.H{"schedules": rows, "total_jp": totalJP})
}

func (h *Handler) ExportJP(c *gin.Context) {
	if !authorize(c) {
		return
	}

	sd := c.Query("start_date")
	ed := c.Query("end_date")
	if sd == "" || ed == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date required"})
		return
	}
	start, err := parseDate(sd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date"})
		return
	}
	end, err := parseDate(ed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date"})
		return
	}

	filter := bson.M{"date": bson.M{"$gte": start, "$lte": end}}
	cur, err := h.Coll.Find(h.Ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var rows []models.Schedule
	if err := cur.All(h.Ctx, &rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type teacherAgg struct {
		NIK     string
		Name    string
		Classes map[string]struct{}
		Weeks   [5]int
		TotalJP int
	}
	data := map[string]*teacherAgg{}

	for _, r := range rows {
		if _, ok := data[r.TeacherNIK]; !ok {
			data[r.TeacherNIK] = &teacherAgg{
				NIK:     r.TeacherNIK,
				Name:    r.TeacherName,
				Classes: map[string]struct{}{},
			}
		}
		ag := data[r.TeacherNIK]

		ag.Classes[r.ClassCode] = struct{}{}

		day := r.Date.Day()
		week := (day-1)/7 + 1
		if week < 1 {
			week = 1
		}
		if week > 5 {
			week = 5
		}
		ag.Weeks[week-1]++
		ag.TotalJP++
	}

	f := excelize.NewFile()
	sheet := "RekapJP"
	f.SetSheetName("Sheet1", sheet)

	f.SetCellValue(sheet, "A1", "No")
	f.SetCellValue(sheet, "B1", "NIK")
	f.SetCellValue(sheet, "C1", "Nama Pengajar")
	f.SetCellValue(sheet, "D1", "Kelas yg Diajar")

	f.MergeCell(sheet, "E1", "I1")
	f.SetCellValue(sheet, "E1", "Total Jam Pelajaran Per Pekan")

	f.SetCellValue(sheet, "J1", "Total JP")

	f.SetCellValue(sheet, "E2", "Pekan 1")
	f.SetCellValue(sheet, "F2", "Pekan 2")
	f.SetCellValue(sheet, "G2", "Pekan 3")
	f.SetCellValue(sheet, "H2", "Pekan 4")
	f.SetCellValue(sheet, "I2", "Pekan 5")

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#FFFF00"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	f.SetCellStyle(sheet, "A1", "D1", headerStyle)
	f.SetCellStyle(sheet, "E1", "E1", headerStyle) 
	f.SetCellStyle(sheet, "J1", "J1", headerStyle)
	f.SetCellStyle(sheet, "A2", "J2", headerStyle)

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rowIndex := 3

	for _, nik := range keys {
		ag := data[nik]

		classList := make([]string, 0, len(ag.Classes))
		for cc := range ag.Classes {
			classList = append(classList, cc)
		}
		sort.Strings(classList)
		classesStr := strings.Join(classList, ", ")

		f.SetCellValue(sheet, fmt.Sprintf("A%d", rowIndex), rowIndex-2)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", rowIndex), ag.NIK)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", rowIndex), ag.Name)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", rowIndex), classesStr)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", rowIndex), ag.Weeks[0])
		f.SetCellValue(sheet, fmt.Sprintf("F%d", rowIndex), ag.Weeks[1])
		f.SetCellValue(sheet, fmt.Sprintf("G%d", rowIndex), ag.Weeks[2])
		f.SetCellValue(sheet, fmt.Sprintf("H%d", rowIndex), ag.Weeks[3])
		f.SetCellValue(sheet, fmt.Sprintf("I%d", rowIndex), ag.Weeks[4])
		f.SetCellValue(sheet, fmt.Sprintf("J%d", rowIndex), ag.TotalJP)

		rowIndex++
	}

	for col := 1; col <= 10; col++ {
		colName, _ := excelize.ColumnNumberToName(col)
		f.SetColWidth(sheet, colName, colName, 18)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="rekap_jp.xlsx"`)
	c.Writer.Write(buf.Bytes())
}

func (h *Handler) ImportExcel(c *gin.Context) {
	if !authorize(c) {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	xl, err := excelize.OpenReader(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid excel file"})
		return
	}
	rows, err := xl.GetRows("Sheet1")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sheet Sheet1 not found"})
		return
	}

	inserted := 0
	failures := []string{}

	for i, row := range rows {
		if i == 0 {
			continue 
		}

		if len(row) < 9 {
			failures = append(failures, fmt.Sprintf("row %d: not enough columns", i+1))
			continue
		}

		date, err := parseDate(strings.TrimSpace(row[5]))
		if err != nil {
			failures = append(failures, fmt.Sprintf("row %d: invalid date %s", i+1, row[5]))
			continue
		}
		jk, err := strconv.Atoi(strings.TrimSpace(row[6]))
		if err != nil {
			failures = append(failures, fmt.Sprintf("row %d: invalid jam_ke %s", i+1, row[6]))
			continue
		}
		classCode := strings.TrimSpace(row[0])
		className := strings.TrimSpace(row[1])
		subjectCode := strings.TrimSpace(row[2])
		teacherNIK := strings.TrimSpace(row[3])
		teacherName := strings.TrimSpace(row[4])
		timeStart := strings.TrimSpace(row[7])
		timeEnd := strings.TrimSpace(row[8])

		conflict, err := h.DetectConflict(date, classCode, teacherNIK, timeStart, timeEnd, "")
		if err != nil {
			failures = append(failures, fmt.Sprintf("row %d: error checking conflict %v", i+1, err))
			continue
		}
		if conflict {
			failures = append(failures, fmt.Sprintf("row %d: conflict detected", i+1))
			continue
		}

		now := time.Now()
		s := &models.Schedule{
			UUID:        uuid.New().String(),
			ClassCode:   classCode,
			ClassName:   className,
			SubjectCode: subjectCode,
			TeacherNIK:  teacherNIK,
			TeacherName: teacherName,
			Date:        date,
			JamKe:       jk,
			TimeStart:   timeStart,
			TimeEnd:     timeEnd,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := h.Repo.Insert(s); err != nil {
			failures = append(failures, fmt.Sprintf("row %d: insert error %v", i+1, err))
			continue
		}
		inserted++
	}

	if len(failures) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Upload sukses, %d baris data ditambahkan.", inserted)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("Upload selesai, %d baris data ditambahkan, %d baris gagal.", inserted, len(failures)),
		"failures": failures,
	})
}
