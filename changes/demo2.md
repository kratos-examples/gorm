# Changes

Code differences compared to source project.

## cmd/demo2kratos/wire_gen.go (+1 -5)

```diff
@@ -28,11 +28,7 @@
 	if err != nil {
 		return nil, nil, err
 	}
-	articleUsecase, err := biz.NewArticleUsecase(dataData, logger)
-	if err != nil {
-		cleanup()
-		return nil, nil, err
-	}
+	articleUsecase := biz.NewArticleUsecase(dataData, logger)
 	articleService := service.NewArticleService(articleUsecase)
 	grpcServer := server.NewGRPCServer(confServer, articleService, logger)
 	httpServer := server.NewHTTPServer(confServer, articleService, logger)
```

## internal/biz/article.go (+156 -106)

```diff
@@ -2,74 +2,83 @@
 
 import (
 	"context"
-	"errors"
 	"log/slog"
 
+	"github.com/go-kratos/kratos/v3/errors"
+	"github.com/yylego/gormcnm"
+	"github.com/yylego/gormrepo"
+	"github.com/yylego/gormrepo/gormclass"
 	"github.com/yylego/kratos-ebz/ebzkratos"
 	pb "github.com/yylego/kratos-examples/demo2kratos/api/article"
 	"github.com/yylego/kratos-examples/demo2kratos/internal/data"
+	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
+	"github.com/yylego/kratos-gorm/gormkratos"
 	"github.com/yylego/must"
 	"gorm.io/gorm"
 	"gorm.io/gorm/clause"
 )
 
-// Article is the GORM type mapped to the "articles" table. This service owns
-// the table; demo1kratos keeps a duplicate of it just to cascade-delete a
-// student's articles (the two services share one database).
-//
-// Article 是映射到 articles 表的 GORM 模型，本服务是这张表的归属方；
-// demo1kratos 里有一份镜像，仅用于删学生时顺带删文章（两服务共用一个库）
 type Article struct {
-	ID        int64  `gorm:"primaryKey;autoIncrement"`
-	Title     string `gorm:"size:256;not null"`
-	Content   string `gorm:"type:text"`
-	StudentID int64  `gorm:"index"`
+	ID        int64
+	Title     string
+	Content   string
+	StudentID int64
 }
 
-func (Article) TableName() string { return "articles" }
-
 type ArticleUsecase struct {
 	data *data.Data
-	slog *slog.Logger
+	repo *gormrepo.Repo[models.Article, *models.ArticleColumns]
+	// The mirrored student repo backs the existence check; the two services share one database.
+	// 镜像的学生 repo 用于存在性校验；两个服务共用一个库。
+	repoStudent *gormrepo.Repo[models.Student, *models.StudentColumns]
+	log         *slog.Logger
 }
 
-func NewArticleUsecase(data *data.Data, logger *slog.Logger) (*ArticleUsecase, error) {
-	// Migrate the owned table plus the mirrored students table (needed in the
-	// existence check); both services share one database
-	// 建好本服务拥有的 articles 表，外加镜像的 students 表（供存在性校验用）
-	if err := data.DB().AutoMigrate(&Article{}, &Student{}); err != nil {
-		return nil, err
+func NewArticleUsecase(data *data.Data, logger *slog.Logger) *ArticleUsecase {
+	return &ArticleUsecase{
+		data:        data,
+		repo:        gormrepo.NewRepo(gormclass.Use(&models.Article{})),
+		repoStudent: gormrepo.NewRepo(gormclass.Use(&models.Student{})),
+		log:         logger,
 	}
-	return &ArticleUsecase{data: data, slog: logger}, nil
 }
 
 func (uc *ArticleUsecase) CreateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
 	must.Nice(a.Title)
 	must.True(a.StudentID > 0)
 
-	// Lock the student row and insert the article in one transaction: the FOR
-	// SHARE lock blocks a concurrent DeleteStudent (which takes FOR UPDATE) from
-	// removing this student before we commit, so we cannot end up with an article
-	// pointing at a student that's being deleted.
-	// 在一个事务里锁住学生行再插入文章：FOR SHARE 锁会挡住并发的 DeleteStudent
-	// （它持 FOR UPDATE）在本事务提交前删除该学生，从而绝不会创建出指向
-	// "正在被删除的学生"的文章
-	res := &Article{Title: a.Title, Content: a.Content, StudentID: a.StudentID}
-	err := uc.data.DB().WithContext(ctx).Transaction(func(db *gorm.DB) error {
-		var student Student
-		if err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).First(&student, a.StudentID).Error; err != nil {
-			return err
+	db := uc.data.DB()
+
+	// Translate the stump: FOR SHARE lock the student row inside the transaction, then insert.
+	// The lock blocks a concurrent DeleteStudent (FOR UPDATE), so we never point an article at a student being deleted.
+	// 翻译桩子：事务里 FOR SHARE 锁住学生行再插文章，挡住并发的 DeleteStudent，绝不指向"正被删的学生"。
+	var article *models.Article
+	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
+		if _, erb := uc.repoStudent.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+			return db.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).Where(cls.ID.Eq(uint(a.StudentID)))
+		}); erb != nil {
+			if erb.NotExist {
+				return pb.ErrorBadParam("student %d does not exist", a.StudentID)
+			}
+			return pb.ErrorDbError("get student: %v", erb.Cause)
 		}
-		return db.Create(res).Error
-	})
-	if err != nil {
-		if errors.Is(err, gorm.ErrRecordNotFound) {
-			return nil, ebzkratos.New(pb.ErrorBadParam("student %d does not exist", a.StudentID))
+		article = &models.Article{Title: a.Title, Content: a.Content, StudentID: a.StudentID}
+		if err := uc.repo.With(ctx, db).Create(article); err != nil {
+			return pb.ErrorArticleCreateFailure("create article: %v", err)
 		}
-		return nil, ebzkratos.New(pb.ErrorArticleCreateFailure("create article: %v", err))
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorTxError("tx: %v", err))
 	}
-	uc.slog.InfoContext(ctx, "created article", "id", res.ID, "student_id", res.StudentID)
-	return res, nil
+	return &Article{
+		ID:        int64(article.ID),
+		Title:     article.Title,
+		Content:   article.Content,
+		StudentID: article.StudentID,
+	}, nil
 }
 
 func (uc *ArticleUsecase) UpdateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
@@ -77,71 +86,87 @@
 	must.Nice(a.Title)
 	must.True(a.StudentID > 0)
 
-	// Same transaction + FOR SHARE lock as CreateArticle: the (new) owning
-	// student cannot be deleted while we re-point the article.
-	// 与 CreateArticle 相同的事务 + FOR SHARE 锁：改文章归属期间，新归属的学生不会被并发删除
-	res := &Article{ID: a.ID}
-	var studentMissing, articleMissing bool
-	err := uc.data.DB().WithContext(ctx).Transaction(func(db *gorm.DB) error {
-		var student Student
-		if err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).First(&student, a.StudentID).Error; err != nil {
-			if errors.Is(err, gorm.ErrRecordNotFound) {
-				studentMissing = true
-				return nil
+	db := uc.data.DB()
+
+	// Same FOR SHARE lock on the new owning student, plus confirm the article exists.
+	// 与创建相同的 FOR SHARE 锁住新归属学生，再确认文章本身存在。
+	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
+		if _, erb := uc.repoStudent.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
+			return db.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).Where(cls.ID.Eq(uint(a.StudentID)))
+		}); erb != nil {
+			if erb.NotExist {
+				return pb.ErrorBadParam("student %d does not exist", a.StudentID)
 			}
-			return err
+			return pb.ErrorDbError("get student: %v", erb.Cause)
 		}
-		upd := db.Model(res).Updates(map[string]any{
-			"title":      a.Title,
-			"content":    a.Content,
-			"student_id": a.StudentID,
-		})
-		if upd.Error != nil {
-			return upd.Error
+		if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+			return db.Where(cls.ID.Eq(uint(a.ID)))
+		}); erb != nil {
+			if erb.NotExist {
+				return pb.ErrorArticleNotFound("article %d not found", a.ID)
+			}
+			return pb.ErrorDbError("get article: %v", erb.Cause)
 		}
-		if upd.RowsAffected == 0 {
-			articleMissing = true
-			return nil
+		if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+			return db.Where(cls.ID.Eq(uint(a.ID)))
+		}, func(cls *models.ArticleColumns) gormcnm.ColumnValueMap {
+			return cls.Kw(cls.Title.Kv(a.Title)).Kw(cls.Content.Kv(a.Content)).Kw(cls.StudentID.Kv(a.StudentID))
+		}); err != nil {
+			return pb.ErrorDbError("update article: %v", err)
 		}
-		return db.First(res, a.ID).Error
-	})
-	if err != nil {
-		return nil, ebzkratos.New(pb.ErrorDbError("update article: %v", err))
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorTxError("tx: %v", err))
 	}
-	if studentMissing {
-		return nil, ebzkratos.New(pb.ErrorBadParam("student %d does not exist", a.StudentID))
-	}
-	if articleMissing {
-		return nil, ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", a.ID))
-	}
-	return res, nil
+	return a, nil
 }
 
 func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, id int64) *ebzkratos.Ebz {
 	must.True(id > 0)
 
-	del := uc.data.DB().WithContext(ctx).Delete(&Article{}, id)
-	if del.Error != nil {
-		return ebzkratos.New(pb.ErrorDbError("delete article: %v", del.Error))
+	db := uc.data.DB()
+
+	if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	}); erb != nil {
+		if erb.NotExist {
+			return ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
+		}
+		return ebzkratos.New(pb.ErrorDbError("get article: %v", erb.Cause))
 	}
-	if del.RowsAffected == 0 {
-		return ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
+
+	if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	}); err != nil {
+		return ebzkratos.New(pb.ErrorDbError("delete article: %v", err))
 	}
-	uc.slog.InfoContext(ctx, "deleted article", "id", id)
 	return nil
 }
 
 func (uc *ArticleUsecase) GetArticle(ctx context.Context, id int64) (*Article, *ebzkratos.Ebz) {
 	must.True(id > 0)
 
-	res := &Article{}
-	if err := uc.data.DB().WithContext(ctx).First(res, id).Error; err != nil {
-		if errors.Is(err, gorm.ErrRecordNotFound) {
+	db := uc.data.DB()
+
+	article, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	})
+	if erb != nil {
+		if erb.NotExist {
 			return nil, ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
 		}
-		return nil, ebzkratos.New(pb.ErrorDbError("get article: %v", err))
+		return nil, ebzkratos.New(pb.ErrorDbError("get article: %v", erb.Cause))
 	}
-	return res, nil
+
+	return &Article{
+		ID:        int64(article.ID),
+		Title:     article.Title,
+		Content:   article.Content,
+		StudentID: article.StudentID,
+	}, nil
 }
 
 func (uc *ArticleUsecase) ListArticles(ctx context.Context, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
@@ -152,26 +177,31 @@
 		pageSize = 10
 	}
 
-	db := uc.data.DB().WithContext(ctx)
+	db := uc.data.DB()
 
-	var total int64
-	if err := db.Model(&Article{}).Count(&total).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("count articles: %v", err))
-	}
-
-	var items []*Article
-	if err := db.Order("id").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&items).Error; err != nil {
+	articles, total, err := uc.repo.With(ctx, db).FindPageAndCount(
+		func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+			return db
+		},
+		func(cls *models.ArticleColumns) gormcnm.OrderByBottle {
+			return cls.ID.Ob("asc")
+		},
+		&gormrepo.Pagination{
+			Offset: int((page - 1) * pageSize),
+			Limit:  int(pageSize),
+		},
+	)
+	if err != nil {
 		return nil, 0, ebzkratos.New(pb.ErrorDbError("list articles: %v", err))
 	}
-	return items, int32(total), nil
+
+	return toArticleItems(articles), int32(total), nil
 }
 
 // ListStudentArticles returns one student's articles, one page at a time. The
-// student↔article relationship gets its own endpoint instead of overloading
-// ListArticles with an extra flag.
+// student↔article relationship gets its own endpoint instead of overloading ListArticles.
 //
-// ListStudentArticles 分页返回某个学生的文章。学生↔文章这层关系单独开一个接口，
-// 而不是往 ListArticles 上塞过滤参数。
+// ListStudentArticles 分页返回某个学生的文章，关系单独开一个接口。
 func (uc *ArticleUsecase) ListStudentArticles(ctx context.Context, studentID int64, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
 	must.True(studentID > 0)
 	if page < 1 {
@@ -181,16 +211,36 @@
 		pageSize = 10
 	}
 
-	db := uc.data.DB().WithContext(ctx)
+	db := uc.data.DB()
 
-	var total int64
-	if err := db.Model(&Article{}).Where("student_id = ?", studentID).Count(&total).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("count student articles: %v", err))
+	articles, total, err := uc.repo.With(ctx, db).FindPageAndCount(
+		func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+			return db.Where(cls.StudentID.Eq(studentID))
+		},
+		func(cls *models.ArticleColumns) gormcnm.OrderByBottle {
+			return cls.ID.Ob("asc")
+		},
+		&gormrepo.Pagination{
+			Offset: int((page - 1) * pageSize),
+			Limit:  int(pageSize),
+		},
+	)
+	if err != nil {
+		return nil, 0, ebzkratos.New(pb.ErrorDbError("list student articles: %v", err))
 	}
 
-	var items []*Article
-	if err := db.Where("student_id = ?", studentID).Order("id").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&items).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("list student articles: %v", err))
+	return toArticleItems(articles), int32(total), nil
+}
+
+func toArticleItems(articles []*models.Article) []*Article {
+	items := make([]*Article, 0, len(articles))
+	for _, v := range articles {
+		items = append(items, &Article{
+			ID:        int64(v.ID),
+			Title:     v.Title,
+			Content:   v.Content,
+			StudentID: v.StudentID,
+		})
 	}
-	return items, int32(total), nil
+	return items
 }
```

## internal/data/data.go (+9 -8)

```diff
@@ -5,6 +5,7 @@
 
 	"github.com/google/wire"
 	"github.com/yylego/kratos-examples/demo2kratos/internal/conf"
+	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
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
+	must.Done(db.AutoMigrate(&models.Article{}, &models.Student{}))
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

## internal/pkg/models/article.go (+14 -0)

```diff
@@ -0,0 +1,14 @@
+package models
+
+import "gorm.io/gorm"
+
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
+	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
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
+		&models.Article{},
+		&models.Student{},
+	}
+
+	// Configure generation options with latest best practices
+	options := gormcngen.NewOptions().
+		WithColumnClassExportable(true). // Generate exportable column class names like ArticleColumns
+		WithColumnsMethodRecvName("c").  // Set receiver name for column methods
+		WithColumnsCheckFieldType(true)  // Enable field type checking for type safe
+
+	// Create configuration and generate code to target file
+	cfg := gormcngen.NewConfigs(objects, options, absPath)
+	cfg.Gen() // Generate code to "gormcnm.gen.go" file
+}
```

## internal/pkg/models/student.go (+20 -0)

```diff
@@ -0,0 +1,20 @@
+package models
+
+import "gorm.io/gorm"
+
+// Student mirrors demo1kratos's students table. This is the article service, so
+// it does not own students — it keeps this duplicate just to check a student
+// exists before creating an article (the two services share one database).
+//
+// Student 与 demo1kratos 的 students 表结构一致。这里是文章服务、不拥有学生表，
+// 保留这份镜像仅用于建文章前校验学生存在（两服务共用一个库）。
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

