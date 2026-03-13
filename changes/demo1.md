# Changes

Code differences compared to source project.

## internal/biz/student.go (+67 -5)

```diff
@@ -4,10 +4,16 @@
 	"context"
 
 	"github.com/brianvoe/gofakeit/v7"
+	"github.com/go-kratos/kratos/v2/errors"
 	"github.com/go-kratos/kratos/v2/log"
+	"github.com/yylego/gormrepo"
+	"github.com/yylego/gormrepo/gormclass"
 	"github.com/yylego/kratos-ebz/ebzkratos"
 	pb "github.com/yylego/kratos-examples/demo1kratos/api/student"
 	"github.com/yylego/kratos-examples/demo1kratos/internal/data"
+	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
+	"github.com/yylego/kratos-gorm/gormkratos"
+	"gorm.io/gorm"
 )
 
 type Student struct {
@@ -19,11 +25,18 @@
 
 type StudentUsecase struct {
 	data *data.Data
+	// Embed a generic repo instance to demo gormrepo usage
+	// In practice, this repo can replace repetitive CRUD code
+	repo *gormrepo.Repo[models.Student, *models.StudentColumns]
 	log  *log.Helper
 }
 
 func NewStudentUsecase(data *data.Data, logger log.Logger) *StudentUsecase {
-	return &StudentUsecase{data: data, log: log.NewHelper(logger)}
+	return &StudentUsecase{
+		data: data,
+		repo: gormrepo.NewRepo(gormclass.Use(&models.Student{})),
+		log:  log.NewHelper(logger),
+	}
 }
 
 func (uc *StudentUsecase) CreateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
@@ -31,6 +44,42 @@
 	if err := gofakeit.Struct(&res); err != nil {
 		return nil, ebzkratos.New(pb.ErrorStudentCreateFailure("fake: %v", err))
 	}
+
+	db := uc.data.DB()
+
+	// This demonstrates how to handle database transactions in Kratos framework
+	//
+	// IMPORTANT: Two-Errors Return Pattern
+	// The gormkratos.Transaction function returns two errors:
+	// - erk: Business logic errors (Kratos framework errors)
+	// - err: Database transaction errors
+	//
+	// When erk != nil, err is always != nil (business error triggers transaction rollback).
+	// So check err first as the main condition, then check erk inside.
+	// When erk != nil, it contains the specific business reason.
+	// Return erk first since it has more business context (reason and code) than what the raw transaction throws.
+	//
+	// Recommended usage pattern (MUST follow):
+	//   if erk, err := gormkratos.Transaction(...); err != nil {
+	//       if erk != nil {
+	//           return erk  // Business error caused rollback
+	//       }
+	//       return WrapTxError(err)  // Database commit failed
+	//   }
+	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
+		record := &models.Student{
+			Name: res.Name,
+		}
+		if err := uc.repo.With(ctx, db).Create(record); err != nil {
+			return errors.New(500, "DB_ERROR", err.Error())
+		}
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorServerError("tx: %v", err))
+	}
 	return &res, nil
 }
 
@@ -47,11 +96,24 @@
 }
 
 func (uc *StudentUsecase) GetStudent(ctx context.Context, id int64) (*Student, *ebzkratos.Ebz) {
-	var res Student
-	if err := gofakeit.Struct(&res); err != nil {
-		return nil, ebzkratos.New(pb.ErrorServerError("fake: %v", err))
+	db := uc.data.DB()
+
+	// Use gormrepo with type-safe column reference
+	// The cls param provides compile-time safe column access
+	record, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	})
+	if erb != nil {
+		if erb.NotExist {
+			return nil, ebzkratos.New(pb.ErrorServerError("not found: %v", erb.Cause))
+		}
+		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
 	}
-	return &res, nil
+
+	return &Student{
+		ID:   int64(record.ID),
+		Name: record.Name,
+	}, nil
 }
 
 func (uc *StudentUsecase) ListStudents(ctx context.Context, page int32, pageSize int32) ([]*Student, int32, *ebzkratos.Ebz) {
```

## internal/data/data.go (+14 -3)

```diff
@@ -4,10 +4,12 @@
 	"github.com/go-kratos/kratos/v2/log"
 	"github.com/google/wire"
 	"github.com/yylego/kratos-examples/demo1kratos/internal/conf"
+	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
 	"github.com/yylego/must"
 	"github.com/yylego/rese"
 	"gorm.io/driver/sqlite"
 	"gorm.io/gorm"
+	loggergorm "gorm.io/gorm/logger"
 )
 
 var ProviderSet = wire.NewSet(NewData)
@@ -17,11 +19,20 @@
 }
 
 func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
-	must.Same(c.Database.Driver, "sqlite3")
-	db := rese.P1(gorm.Open(sqlite.Open(c.Database.Source), &gorm.Config{}))
+	dsn := must.Nice(c.Database.Source)
+	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{
+		Logger: loggergorm.Default.LogMode(loggergorm.Info),
+	}))
+
+	must.Done(db.AutoMigrate(&models.Student{}))
+
 	cleanup := func() {
 		log.NewHelper(logger).Info("closing the data resources")
-		_ = rese.P1(db.DB()).Close()
+		must.Done(rese.P1(db.DB()).Close())
 	}
 	return &Data{db: db}, cleanup, nil
+}
+
+func (d *Data) DB() *gorm.DB {
+	return d.db
 }
```

## internal/pkg/models/gormcnm.gen.go (+41 -0)

```diff
@@ -0,0 +1,41 @@
+// Code generated using gormcngen. DO NOT EDIT.
+// This file was auto generated via github.com/yylego/gormcngen
+
+//go:build !gormcngen_generate
+
+// Generated from: gormcnm.gen_test.go:34 -> models_test.TestGenerateColumns
+// ========== GORMCNGEN:DO-NOT-EDIT-MARKER:END ==========
+
+// Code generated using gormcngen. DO NOT EDIT.
+// This file was auto generated via github.com/yylego/gormcngen
+
+package models
+
+import (
+	"time"
+
+	"github.com/yylego/gormcnm"
+	"gorm.io/gorm"
+)
+
+func (c *Student) Columns() *StudentColumns {
+	return &StudentColumns{
+		// Auto-generated: column names and types mapping. DO NOT EDIT. // 自动生成：列名和类型映射。请勿编辑。
+		ID:        gormcnm.Cnm(c.ID, "id"),
+		CreatedAt: gormcnm.Cnm(c.CreatedAt, "created_at"),
+		UpdatedAt: gormcnm.Cnm(c.UpdatedAt, "updated_at"),
+		DeletedAt: gormcnm.Cnm(c.DeletedAt, "deleted_at"),
+		Name:      gormcnm.Cnm(c.Name, "name"),
+	}
+}
+
+type StudentColumns struct {
+	// Auto-generated: embedding operation functions to make it simple to use. DO NOT EDIT. // 自动生成：嵌入操作函数便于使用。请勿编辑。
+	gormcnm.ColumnOperationClass
+	// Auto-generated: column names and types in database table. DO NOT EDIT. // 自动生成：数据库表的列名和类型。请勿编辑。
+	ID        gormcnm.ColumnName[uint]
+	CreatedAt gormcnm.ColumnName[time.Time]
+	UpdatedAt gormcnm.ColumnName[time.Time]
+	DeletedAt gormcnm.ColumnName[gorm.DeletedAt]
+	Name      gormcnm.ColumnName[string]
+}
```

## internal/pkg/models/gormcnm.gen_test.go (+36 -0)

```diff
@@ -0,0 +1,36 @@
+package models_test
+
+import (
+	"testing"
+
+	"github.com/yylego/gormcngen"
+	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
+	"github.com/yylego/osexistpath/osmustexist"
+	"github.com/yylego/runpath/runtestpath"
+)
+
+// Auto generate columns with go generate command
+// Support execution via: go generate ./...
+// Delete this comment block if auto generation is not needed
+//
+//go:generate go test -v -run TestGenerateColumns
+func TestGenerateColumns(t *testing.T) {
+	// Retrieve the absolute path of the source file based on current test file location
+	absPath := osmustexist.FILE(runtestpath.SrcPath(t))
+	t.Log(absPath)
+
+	// Define data objects used in column generation - supports both instance and non-instance types
+	objects := []any{
+		&models.Student{},
+	}
+
+	// Configure generation options with latest best practices
+	options := gormcngen.NewOptions().
+		WithColumnClassExportable(true). // Generate exportable column class names like StudentColumns
+		WithColumnsMethodRecvName("c").  // Set receiver name for column methods
+		WithColumnsCheckFieldType(true)  // Enable field type checking for type safe
+
+	// Create configuration and generate code to target file
+	cfg := gormcngen.NewConfigs(objects, options, absPath)
+	cfg.Gen() // Generate code to "gormcnm.gen.go" file
+}
```

## internal/pkg/models/student.go (+12 -0)

```diff
@@ -0,0 +1,12 @@
+package models
+
+import "gorm.io/gorm"
+
+type Student struct {
+	gorm.Model
+	Name string `gorm:"type:varchar(255)"`
+}
+
+func (*Student) TableName() string {
+	return "students"
+}
```

