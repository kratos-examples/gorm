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

## internal/biz/student.go (+132 -75)

```diff
@@ -2,132 +2,170 @@
 
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
 	"gorm.io/gorm/clause"
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
+	repo *gormrepo.Repo[models.Student, *models.StudentColumns]
+	// The mirrored article repo backs the cascade delete; the two services share one database.
+	// 镜像的文章 repo 用于级联删除；两个服务共用一个库。
+	repoArticle *gormrepo.Repo[models.Article, *models.ArticleColumns]
+	log         *slog.Logger
 }
 
-func NewStudentUsecase(data *data.Data, logger *slog.Logger) (*StudentUsecase, error) {
-	// Share one database with the article service: keep both tables in sync here
-	// 与文章服务共用一个库：在这里把两张表都建好
-	if err := data.DB().AutoMigrate(&Student{}, &Article{}); err != nil {
-		return nil, err
+func NewStudentUsecase(data *data.Data, logger *slog.Logger) *StudentUsecase {
+	return &StudentUsecase{
+		data:        data,
+		repo:        gormrepo.NewRepo(gormclass.Use(&models.Student{})),
+		repoArticle: gormrepo.NewRepo(gormclass.Use(&models.Article{})),
+		log:         logger,
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
+	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
+		student = &models.Student{
+			Name:      s.Name,
+			Age:       s.Age,
+			ClassName: s.ClassName,
+		}
+		if err := uc.repo.With(ctx, db).Create(student); err != nil {
+			return pb.ErrorStudentCreateFailure("create student: %v", err)
+		}
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorTxError("tx: %v", err))
 	}
-	uc.slog.InfoContext(ctx, "created student", "id", res.ID, "name", res.Name)
-	return res, nil
+	return &Student{
+		ID:        int64(student.ID),
+		Name:      student.Name,
+		Age:       student.Age,
+		ClassName: student.ClassName,
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
+	// Confirm the student exists first, matching the stump: a missing row yields StudentNotFound.
+	// 先确认学生存在，对齐桩子：查不到返回 StudentNotFound 而非静默成功。
+	if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(s.ID)))
+	}); erb != nil {
+		if erb.NotExist {
+			return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", s.ID))
+		}
+		return nil, ebzkratos.New(pb.ErrorDbError("get student: %v", erb.Cause))
 	}
-	if upd.RowsAffected == 0 {
-		return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", s.ID))
+
+	if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(s.ID)))
+	}, func(cls *models.StudentColumns) gormcnm.ColumnValueMap {
+		return cls.Kw(cls.Name.Kv(s.Name)).Kw(cls.Age.Kv(s.Age)).Kw(cls.ClassName.Kv(s.ClassName))
+	}); err != nil {
+		return nil, ebzkratos.New(pb.ErrorDbError("update student: %v", err))
 	}
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
+	db := uc.data.DB()
+
+	// Translate the stump's atomic cascade delete, in one transaction:
+	//   ① FOR UPDATE lock the student row (a concurrent CreateArticle holds FOR SHARE, so the two serialize);
+	//   ② delete the student's articles (children first);
+	//   ③ delete the student (parent last).
+	// 翻译桩子的原子级联删除，全在一个事务里：①FOR UPDATE 锁学生行 ②先删文章 ③再删学生。
 	var notFound bool
-	var removedArticles int64
-	err := uc.data.DB().WithContext(ctx).Transaction(func(db *gorm.DB) error {
-		var s Student
-		if err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).First(&s, id).Error; err != nil {
-			if errors.Is(err, gorm.ErrRecordNotFound) {
+	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
+		if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+			return db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where(cls.ID.Eq(uint(id)))
+		}); erb != nil {
+			if erb.NotExist {
 				notFound = true
 				return nil
 			}
-			return err
+			return pb.ErrorDbError("get student: %v", erb.Cause)
 		}
-		del := db.Where("student_id = ?", id).Delete(&Article{})
-		if del.Error != nil {
-			return del.Error
+		if err := uc.repoArticle.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+			return db.Where(cls.StudentID.Eq(id))
+		}); err != nil {
+			return pb.ErrorDbError("delete articles: %v", err)
 		}
-		removedArticles = del.RowsAffected
-		return db.Delete(&Student{}, id).Error
-	})
-	if err != nil {
+		if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+			return db.Where(cls.ID.Eq(uint(id)))
+		}); err != nil {
+			return pb.ErrorDbError("delete student: %v", err)
+		}
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return ebzkratos.New(erk)
+		}
 		return ebzkratos.New(pb.ErrorTxError("delete student with articles: %v", err))
 	}
 	if notFound {
 		return ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", id))
 	}
-	uc.slog.InfoContext(ctx, "deleted student and cascaded articles", "student_id", id, "articles_removed", removedArticles)
 	return nil
 }
 
 func (uc *StudentUsecase) GetStudent(ctx context.Context, id int64) (*Student, *ebzkratos.Ebz) {
 	must.True(id > 0)
 
-	res := &Student{}
-	if err := uc.data.DB().WithContext(ctx).First(res, id).Error; err != nil {
-		if errors.Is(err, gorm.ErrRecordNotFound) {
+	db := uc.data.DB()
+
+	student, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	})
+	if erb != nil {
+		if erb.NotExist {
 			return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", id))
 		}
-		return nil, ebzkratos.New(pb.ErrorDbError("get student: %v", err))
+		return nil, ebzkratos.New(pb.ErrorDbError("get student: %v", erb.Cause))
 	}
-	return res, nil
+
+	return &Student{
+		ID:        int64(student.ID),
+		Name:      student.Name,
+		Age:       student.Age,
+		ClassName: student.ClassName,
+	}, nil
 }
 
 func (uc *StudentUsecase) ListStudents(ctx context.Context, page int32, pageSize int32) ([]*Student, int32, *ebzkratos.Ebz) {
@@ -138,16 +176,35 @@
 		pageSize = 10
 	}
 
-	db := uc.data.DB().WithContext(ctx)
+	db := uc.data.DB()
 
-	var total int64
-	if err := db.Model(&Student{}).Count(&total).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("count students: %v", err))
+	// gormrepo FindPageAndCount returns the page and the row count in one shot,
+	// replacing the stump's hand-written Count + order + offset + limit.
+	// gormrepo 的 FindPageAndCount 一次拿到当页数据和总行数。
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
+		return nil, 0, ebzkratos.New(pb.ErrorDbError("list students: %v", err))
 	}
 
-	var items []*Student
-	if err := db.Order("id").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&items).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("list students: %v", err))
+	items := make([]*Student, 0, len(students))
+	for _, v := range students {
+		items = append(items, &Student{
+			ID:        int64(v.ID),
+			Name:      v.Name,
+			Age:       v.Age,
+			ClassName: v.ClassName,
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
+	must.Done(db.AutoMigrate(&models.Student{}, &models.Article{}))
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

## internal/pkg/models/article.go (+20 -0)

```diff
@@ -0,0 +1,20 @@
+package models
+
+import "gorm.io/gorm"
+
+// Article mirrors demo2kratos's articles table. This is the student service, so
+// it does not own articles — it keeps this duplicate just to cascade-delete a
+// student's articles when the student is removed (the two services share one database).
+//
+// Article 与 demo2kratos 的 articles 表结构一致。这里是学生服务、不拥有文章表，
+// 保留这份镜像仅用于删学生时顺带删掉他名下的文章（两服务共用一个库）。
+type Article struct {
+	gorm.Model
+	Title     string `gorm:"type:varchar(255)"`
+	Content   string `gorm:"type:text"`
+	StudentID int64  `gorm:"index"`
+}
+
+func (*Article) TableName() string {
+	return "articles"
+}
```

## internal/pkg/models/gormcnm.gen.go (+71 -0)

```diff
@@ -0,0 +1,71 @@
+// Code generated using gormcngen. DO NOT EDIT.
+// This file was auto generated via github.com/yylego/gormcngen
+
+//go:build !gormcngen_generate
+
+// Generated from: gormcnm.gen_test.go:35 -> models_test.TestGenerateColumns
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
+		Age:       gormcnm.Cnm(c.Age, "age"),
+		ClassName: gormcnm.Cnm(c.ClassName, "class_name"),
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
+	Age       gormcnm.ColumnName[int32]
+	ClassName gormcnm.ColumnName[string]
+}
+
+func (c *Article) Columns() *ArticleColumns {
+	return &ArticleColumns{
+		// Auto-generated: column names and types mapping. DO NOT EDIT. // 自动生成：列名和类型映射。请勿编辑。
+		ID:        gormcnm.Cnm(c.ID, "id"),
+		CreatedAt: gormcnm.Cnm(c.CreatedAt, "created_at"),
+		UpdatedAt: gormcnm.Cnm(c.UpdatedAt, "updated_at"),
+		DeletedAt: gormcnm.Cnm(c.DeletedAt, "deleted_at"),
+		Title:     gormcnm.Cnm(c.Title, "title"),
+		Content:   gormcnm.Cnm(c.Content, "content"),
+		StudentID: gormcnm.Cnm(c.StudentID, "student_id"),
+	}
+}
+
+type ArticleColumns struct {
+	// Auto-generated: embedding operation functions to make it simple to use. DO NOT EDIT. // 自动生成：嵌入操作函数便于使用。请勿编辑。
+	gormcnm.ColumnOperationClass
+	// Auto-generated: column names and types in database table. DO NOT EDIT. // 自动生成：数据库表的列名和类型。请勿编辑。
+	ID        gormcnm.ColumnName[uint]
+	CreatedAt gormcnm.ColumnName[time.Time]
+	UpdatedAt gormcnm.ColumnName[time.Time]
+	DeletedAt gormcnm.ColumnName[gorm.DeletedAt]
+	Title     gormcnm.ColumnName[string]
+	Content   gormcnm.ColumnName[string]
+	StudentID gormcnm.ColumnName[int64]
+}
```

## internal/pkg/models/gormcnm.gen_test.go (+37 -0)

```diff
@@ -0,0 +1,37 @@
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
+		&models.Article{},
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

## internal/pkg/models/student.go (+14 -0)

```diff
@@ -0,0 +1,14 @@
+package models
+
+import "gorm.io/gorm"
+
+type Student struct {
+	gorm.Model
+	Name      string `gorm:"type:varchar(255)"`
+	Age       int32  `gorm:"type:int"`
+	ClassName string `gorm:"type:varchar(255)"`
+}
+
+func (*Student) TableName() string {
+	return "students"
+}
```

