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

## internal/biz/article.go (+152 -114)

```diff
@@ -2,146 +2,149 @@
 
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
-	"gorm.io/gorm/clause"
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
+	// Embed a generic repo instance to demo gormrepo usage
+	// In practice, this repo can replace repetitive CRUD code
+	repo *gormrepo.Repo[models.Article, *models.ArticleColumns]
+	log  *slog.Logger
 }
 
-func NewArticleUsecase(data *data.Data, logger *slog.Logger) (*ArticleUsecase, error) {
-	// Migrate the owned table plus the mirrored students table (needed in the
-	// existence check); both services share one database
-	// 建好本服务拥有的 articles 表，外加镜像的 students 表（供存在性校验用）
-	if err := data.DB().AutoMigrate(&Article{}, &Student{}); err != nil {
-		return nil, err
+func NewArticleUsecase(data *data.Data, logger *slog.Logger) *ArticleUsecase {
+	return &ArticleUsecase{
+		data: data,
+		repo: gormrepo.NewRepo(gormclass.Use(&models.Article{})),
+		log:  logger,
 	}
-	return &ArticleUsecase{data: data, slog: logger}, nil
 }
 
 func (uc *ArticleUsecase) CreateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
 	must.Nice(a.Title)
-	must.True(a.StudentID > 0)
 
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
+	var article *models.Article
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
+		article = &models.Article{
+			Title:     a.Title,
+			Content:   a.Content,
+			StudentID: a.StudentID,
 		}
-		return db.Create(res).Error
-	})
-	if err != nil {
-		if errors.Is(err, gorm.ErrRecordNotFound) {
-			return nil, ebzkratos.New(pb.ErrorBadParam("student %d does not exist", a.StudentID))
+		if err := uc.repo.With(ctx, db).Create(article); err != nil {
+			return errors.New(500, "DB_ERROR", err.Error())
 		}
-		return nil, ebzkratos.New(pb.ErrorArticleCreateFailure("create article: %v", err))
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorServerError("tx: %v", err))
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
 	must.True(a.ID > 0)
 	must.Nice(a.Title)
-	must.True(a.StudentID > 0)
 
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
-			}
-			return err
-		}
-		upd := db.Model(res).Updates(map[string]any{
-			"title":      a.Title,
-			"content":    a.Content,
-			"student_id": a.StudentID,
-		})
-		if upd.Error != nil {
-			return upd.Error
-		}
-		if upd.RowsAffected == 0 {
-			articleMissing = true
-			return nil
-		}
-		return db.First(res, a.ID).Error
-	})
-	if err != nil {
-		return nil, ebzkratos.New(pb.ErrorDbError("update article: %v", err))
+	db := uc.data.DB()
+
+	// Use gormrepo UpdatesM with type-safe column value map
+	if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(a.ID)))
+	}, func(cls *models.ArticleColumns) gormcnm.ColumnValueMap {
+		return cls.Kw(cls.Title.Kv(a.Title)).Kw(cls.Content.Kv(a.Content))
+	}); err != nil {
+		return nil, ebzkratos.New(pb.ErrorServerError("update: %v", err))
 	}
-	if studentMissing {
-		return nil, ebzkratos.New(pb.ErrorBadParam("student %d does not exist", a.StudentID))
-	}
-	if articleMissing {
-		return nil, ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", a.ID))
-	}
-	return res, nil
+
+	return a, nil
 }
 
 func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, id int64) *ebzkratos.Ebz {
 	must.True(id > 0)
 
-	del := uc.data.DB().WithContext(ctx).Delete(&Article{}, id)
-	if del.Error != nil {
-		return ebzkratos.New(pb.ErrorDbError("delete article: %v", del.Error))
+	db := uc.data.DB()
+
+	// Use gormrepo DeleteW with type-safe where condition
+	if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	}); err != nil {
+		return ebzkratos.New(pb.ErrorServerError("delete: %v", err))
 	}
-	if del.RowsAffected == 0 {
-		return ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
-	}
-	uc.slog.InfoContext(ctx, "deleted article", "id", id)
 	return nil
 }
 
 func (uc *ArticleUsecase) GetArticle(ctx context.Context, id int64) (*Article, *ebzkratos.Ebz) {
 	must.True(id > 0)
 
-	res := &Article{}
-	if err := uc.data.DB().WithContext(ctx).First(res, id).Error; err != nil {
-		if errors.Is(err, gorm.ErrRecordNotFound) {
-			return nil, ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
+	db := uc.data.DB()
+
+	// Use gormrepo with type-safe column reference
+	// The cls param provides compile-time safe column access
+	article, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	})
+	if erb != nil {
+		if erb.NotExist {
+			return nil, ebzkratos.New(pb.ErrorServerError("not found: %v", erb.Cause))
 		}
-		return nil, ebzkratos.New(pb.ErrorDbError("get article: %v", err))
+		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
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
@@ -152,28 +155,43 @@
 		pageSize = 10
 	}
 
-	db := uc.data.DB().WithContext(ctx)
+	db := uc.data.DB()
 
-	var total int64
-	if err := db.Model(&Article{}).Count(&total).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("count articles: %v", err))
+	// gormrepo FindPageAndCount replaces the stump's hand-written Count + Order + Offset + Limit
+	// with one typed call that returns the current page plus the total row count together.
+	// gormrepo 的 FindPageAndCount 把桩子里手写的 Count + Order + Offset + Limit
+	// 收敛成一个类型安全的调用：一次拿到当页数据和总行数
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
+		return nil, 0, ebzkratos.New(pb.ErrorServerError("list: %v", err))
 	}
 
-	var items []*Article
-	if err := db.Order("id").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&items).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("list articles: %v", err))
+	items := make([]*Article, 0, len(articles))
+	for _, v := range articles {
+		items = append(items, &Article{
+			ID:        int64(v.ID),
+			Title:     v.Title,
+			Content:   v.Content,
+			StudentID: v.StudentID,
+		})
 	}
 	return items, int32(total), nil
 }
 
-// ListStudentArticles returns one student's articles, one page at a time. The
-// student↔article relationship gets its own endpoint instead of overloading
-// ListArticles with an extra flag.
-//
-// ListStudentArticles 分页返回某个学生的文章。学生↔文章这层关系单独开一个接口，
-// 而不是往 ListArticles 上塞过滤参数。
 func (uc *ArticleUsecase) ListStudentArticles(ctx context.Context, studentID int64, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
 	must.True(studentID > 0)
+
 	if page < 1 {
 		page = 1
 	}
@@ -181,16 +199,36 @@
 		pageSize = 10
 	}
 
-	db := uc.data.DB().WithContext(ctx)
+	db := uc.data.DB()
 
-	var total int64
-	if err := db.Model(&Article{}).Where("student_id = ?", studentID).Count(&total).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("count student articles: %v", err))
+	// gormrepo FindPageAndCount with a type-safe student_id filter: the paged relational
+	// query stays one typed call instead of the stump's manual Where + Count + Offset + Limit.
+	// gormrepo 的 FindPageAndCount 加类型安全的 student_id 过滤：带分页的关联查询
+	// 仍是一个类型安全的调用，替掉桩子里手写的 Where + Count + Offset + Limit
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
+		return nil, 0, ebzkratos.New(pb.ErrorServerError("list student articles: %v", err))
 	}
 
-	var items []*Article
-	if err := db.Where("student_id = ?", studentID).Order("id").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&items).Error; err != nil {
-		return nil, 0, ebzkratos.New(pb.ErrorDbError("list student articles: %v", err))
+	items := make([]*Article, 0, len(articles))
+	for _, v := range articles {
+		items = append(items, &Article{
+			ID:        int64(v.ID),
+			Title:     v.Title,
+			Content:   v.Content,
+			StudentID: v.StudentID,
+		})
 	}
 	return items, int32(total), nil
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
+	must.Done(db.AutoMigrate(&models.Article{}))
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

## internal/pkg/models/gormcnm.gen.go (+45 -0)

```diff
@@ -0,0 +1,45 @@
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

## internal/pkg/models/gormcnm.gen_test.go (+36 -0)

```diff
@@ -0,0 +1,36 @@
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

