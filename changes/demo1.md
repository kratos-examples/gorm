# Changes

Code differences compared to source project.

## cmd/demo1kratos/wire_gen.go (+1 -5)

```diff
@@ -28,11 +28,7 @@
 	if err != nil {
 		return nil, nil, err
 	}
-	studentUsecase, err := biz.NewStudentUsecase(dataData, logger)
-	if err != nil {
-		cleanup()
-		return nil, nil, err
-	}
+	studentUsecase := biz.NewStudentUsecase(dataData, logger)
 	studentService := service.NewStudentService(studentUsecase)
 	grpcServer := server.NewGRPCServer(confServer, studentService, logger)
 	httpServer := server.NewHTTPServer(confServer, studentService, logger)
```

## internal/biz/student.go (+117 -88)

```diff
@@ -2,132 +2,143 @@
 
 import (
 	"context"
-	"errors"
 	"log/slog"
 
+	"github.com/go-kratos/kratos/v3/errors"
+	"github.com/yylego/gormcnm"
+	"github.com/yylego/gormrepo"
+	"github.com/yylego/gormrepo/gormclass"
 	"github.com/yylego/kratos-ebz/ebzkratos"
 	pb "github.com/yylego/kratos-examples/demo1kratos/api/student"
 	"github.com/yylego/kratos-examples/demo1kratos/internal/data"
+	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
+	"github.com/yylego/kratos-gorm/gormkratos"
 	"github.com/yylego/must"
 	"gorm.io/gorm"
-	"gorm.io/gorm/clause"
 )
 
-// Student is the GORM type mapped to the "students" table.
-//
-// Student 是映射到 students 表的 GORM 模型
 type Student struct {
-	ID        int64  `gorm:"primaryKey;autoIncrement"`
-	Name      string `gorm:"size:128;not null"`
+	ID        int64
+	Name      string
 	Age       int32
-	ClassName string `gorm:"size:128"`
+	ClassName string
 }
 
-func (Student) TableName() string { return "students" }
-
-// The mirrored Article type behind cascade-delete lives in article.go.
-// 用于级联删除的 Article 镜像模型定义在 article.go 中。
-
 type StudentUsecase struct {
 	data *data.Data
-	slog *slog.Logger
+	// Embed a generic repo instance to demo gormrepo usage
+	// In practice, this repo can replace repetitive CRUD code
+	repo *gormrepo.Repo[models.Student, *models.StudentColumns]
+	log  *slog.Logger
 }
 
-func NewStudentUsecase(data *data.Data, logger *slog.Logger) (*StudentUsecase, error) {
-	// Share one database with the article service: keep both tables in sync here
-	// 与文章服务共用一个库：在这里把两张表都建好
-	if err := data.DB().AutoMigrate(&Student{}, &Article{}); err != nil {
-		return nil, err
+func NewStudentUsecase(data *data.Data, logger *slog.Logger) *StudentUsecase {
+	return &StudentUsecase{
+		data: data,
+		repo: gormrepo.NewRepo(gormclass.Use(&models.Student{})),
+		log:  logger,
 	}
-	return &StudentUsecase{data: data, slog: logger}, nil
 }
 
 func (uc *StudentUsecase) CreateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
 	must.Nice(s.Name)
 
-	res := &Student{Name: s.Name, Age: s.Age, ClassName: s.ClassName}
-	if err := uc.data.DB().WithContext(ctx).Create(res).Error; err != nil {
-		return nil, ebzkratos.New(pb.ErrorStudentCreateFailure("create student: %v", err))
+	db := uc.data.DB()
+
+	var student *models.Student
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
+		student = &models.Student{
+			Name: s.Name,
+		}
+		if err := uc.repo.With(ctx, db).Create(student); err != nil {
+			return errors.New(500, "DB_ERROR", err.Error())
+		}
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorServerError("tx: %v", err))
 	}
-	uc.slog.InfoContext(ctx, "created student", "id", res.ID, "name", res.Name)
-	return res, nil
+	return &Student{
+		ID:   int64(student.ID),
+		Name: student.Name,
+	}, nil
 }
 
 func (uc *StudentUsecase) UpdateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
 	must.True(s.ID > 0)
 	must.Nice(s.Name)
 
-	res := &Student{ID: s.ID}
-	upd := uc.data.DB().WithContext(ctx).Model(res).Updates(map[string]any{
-		"name":       s.Name,
-		"age":        s.Age,
-		"class_name": s.ClassName,
-	})
-	if upd.Error != nil {
-		return nil, ebzkratos.New(pb.ErrorDbError("update student: %v", upd.Error))
+	db := uc.data.DB()
+
+	// Use gormrepo UpdatesM with type-safe column value map
+	if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(s.ID)))
+	}, func(cls *models.StudentColumns) gormcnm.ColumnValueMap {
+		return cls.Kw(cls.Name.Kv(s.Name))
+	}); err != nil {
+		return nil, ebzkratos.New(pb.ErrorServerError("update: %v", err))
 	}
-	if upd.RowsAffected == 0 {
-		return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", s.ID))
-	}
-	if err := uc.data.DB().WithContext(ctx).First(res, s.ID).Error; err != nil {
-		return nil, ebzkratos.New(pb.ErrorDbError("reload student: %v", err))
-	}
-	return res, nil
+
+	return s, nil
 }
 
 func (uc *StudentUsecase) DeleteStudent(ctx context.Context, id int64) *ebzkratos.Ebz {
 	must.True(id > 0)
 
-	// Atomic, race-safe cascade delete, in one transaction:
-	//   1. lock the student row (FOR UPDATE) so no article can target
-	//      this student meanwhile — CreateArticle takes a conflicting FOR SHARE
-	//      lock on the same row, so the two operations serialize;
-	//   2. delete the student's articles (children first);
-	//   3. delete the student (parent last).
-	// 原子且并发安全的级联删除，全部在一个事务里完成：
-	//   ① 用 FOR UPDATE 锁住学生行，删除期间不允许给该学生并发新建文章——CreateArticle
-	//      会对同一行加互斥的 FOR SHARE 锁，二者因此串行化；
-	//   ② 先删该学生名下的文章（子表在前）；
-	//   ③ 再删学生本身（父表在后）。
-	var notFound bool
-	var removedArticles int64
-	err := uc.data.DB().WithContext(ctx).Transaction(func(db *gorm.DB) error {
-		var s Student
-		if err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).First(&s, id).Error; err != nil {
-			if errors.Is(err, gorm.ErrRecordNotFound) {
-				notFound = true
-				return nil
-			}
-			return err
-		}
-		del := db.Where("student_id = ?", id).Delete(&Article{})
-		if del.Error != nil {
-			return del.Error
-		}
-		removedArticles = del.RowsAffected
-		return db.Delete(&Student{}, id).Error
-	})
-	if err != nil {
-		return ebzkratos.New(pb.ErrorTxError("delete student with articles: %v", err))
+	db := uc.data.DB()
+
+	// Use gormrepo DeleteW with type-safe where condition
+	if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	}); err != nil {
+		return ebzkratos.New(pb.ErrorServerError("delete: %v", err))
 	}
-	if notFound {
-		return ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", id))
-	}
-	uc.slog.InfoContext(ctx, "deleted student and cascaded articles", "student_id", id, "articles_removed", removedArticles)
 	return nil
 }
 
 func (uc *StudentUsecase) GetStudent(ctx context.Context, id int64) (*Student, *ebzkratos.Ebz) {
 	must.True(id > 0)
 
-	res := &Student{}
-	if err := uc.data.DB().WithContext(ctx).First(res, id).Error; err != nil {
-		if errors.Is(err, gorm.ErrRecordNotFound) {
-			return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", id))
+	db := uc.data.DB()
+
+	// Use gormrepo with type-safe column reference
+	// The cls param provides compile-time safe column access
+	student, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	})
+	if erb != nil {
+		if erb.NotExist {
+			return nil, ebzkratos.New(pb.ErrorServerError("not found: %v", erb.Cause))
 		}
-		return nil, ebzkratos.New(pb.ErrorDbError("get student: %v", err))
+		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
 	}
-	return res, nil
+
+	return &Student{
+		ID:   int64(student.ID),
+		Name: student.Name,
+	}, nil
 }
 
 func (uc *StudentUsecase) ListStudents(ctx context.Context, page int32, pageSize int32) ([]*Student, int32, *ebzkratos.Ebz) {
@@ -138,16 +149,34 @@
 		pageSize = 10
 	}
 
-	db := uc.data.DB().WithContext(ctx)
+	db := uc.data.DB()
 
-	var total int64
-	if err := db.Model(&Student{}).Count(&total).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("count students: %v", err))
+	// gormrepo FindPageAndCount replaces the stump's hand-written Count + Order + Offset + Limit
+	// with one typed call that returns the current page plus the total row count together.
+	// gormrepo 的 FindPageAndCount 把桩子里手写的 Count + Order + Offset + Limit
+	// 收敛成一个类型安全的调用：一次拿到当页数据和总行数
+	students, total, err := uc.repo.With(ctx, db).FindPageAndCount(
+		func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+			return db
+		},
+		func(cls *models.StudentColumns) gormcnm.OrderByBottle {
+			return cls.ID.Ob("asc")
+		},
+		&gormrepo.Pagination{
+			Offset: int((page - 1) * pageSize),
+			Limit:  int(pageSize),
+		},
+	)
+	if err != nil {
+		return nil, 0, ebzkratos.New(pb.ErrorServerError("list: %v", err))
 	}
 
-	var items []*Student
-	if err := db.Order("id").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&items).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("list students: %v", err))
+	items := make([]*Student, 0, len(students))
+	for _, v := range students {
+		items = append(items, &Student{
+			ID:   int64(v.ID),
+			Name: v.Name,
+		})
 	}
 	return items, int32(total), nil
 }
```

## internal/data/data.go (+9 -8)

```diff
@@ -5,6 +5,7 @@
 
 	"github.com/google/wire"
 	"github.com/yylego/kratos-examples/demo1kratos/internal/conf"
+	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
 	"github.com/yylego/must"
 	"github.com/yylego/rese"
 	"gorm.io/driver/postgres"
@@ -17,19 +18,19 @@
 	db *gorm.DB
 }
 
-// DB exposes the underlying gorm handle so the biz code can run true queries.
-//
-// DB 暴露底层 gorm 句柄，供 biz 层执行真实的数据库读写
-func (d *Data) DB() *gorm.DB {
-	return d.db
-}
-
 func NewData(c *conf.Data, logger *slog.Logger) (*Data, func(), error) {
 	must.Same(c.Database.Driver, "postgres")
 	db := rese.P1(gorm.Open(postgres.Open(c.Database.Source), &gorm.Config{}))
+
+	must.Done(db.AutoMigrate(&models.Student{}))
+
 	cleanup := func() {
 		logger.Info("closing the data resources")
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

