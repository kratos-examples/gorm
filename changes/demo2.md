# Changes

Code differences compared to source project.

## internal/biz/article.go (+112 -16)

```diff
@@ -3,12 +3,18 @@
 import (
 	"context"
 
-	"github.com/brianvoe/gofakeit/v7"
+	"github.com/go-kratos/kratos/v2/errors"
 	"github.com/go-kratos/kratos/v2/log"
+	"github.com/yylego/gormcnm"
+	"github.com/yylego/gormrepo"
+	"github.com/yylego/gormrepo/gormclass"
 	"github.com/yylego/kratos-ebz/ebzkratos"
 	pb "github.com/yylego/kratos-examples/demo2kratos/api/article"
 	"github.com/yylego/kratos-examples/demo2kratos/internal/data"
+	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
+	"github.com/yylego/kratos-gorm/gormkratos"
 	"github.com/yylego/must"
+	"gorm.io/gorm"
 )
 
 type Article struct {
@@ -20,52 +26,142 @@
 
 type ArticleUsecase struct {
 	data *data.Data
+	// Embed a generic repo instance to demo gormrepo usage
+	// In practice, this repo can replace repetitive CRUD code
+	repo *gormrepo.Repo[models.Article, *models.ArticleColumns]
 	log  *log.Helper
 }
 
 func NewArticleUsecase(data *data.Data, logger log.Logger) *ArticleUsecase {
-	return &ArticleUsecase{data: data, log: log.NewHelper(logger)}
+	return &ArticleUsecase{
+		data: data,
+		repo: gormrepo.NewRepo(gormclass.Use(&models.Article{})),
+		log:  log.NewHelper(logger),
+	}
 }
 
 func (uc *ArticleUsecase) CreateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
 	must.Nice(a.Title)
 
-	var res Article
-	if err := gofakeit.Struct(&res); err != nil {
-		return nil, ebzkratos.New(pb.ErrorArticleCreateFailure("fake: %v", err))
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
+			Title:   a.Title,
+			Content: a.Content,
+		}
+		if err := uc.repo.With(ctx, db).Create(article); err != nil {
+			return errors.New(500, "DB_ERROR", err.Error())
+		}
+		return nil
+	}); err != nil {
+		if erk != nil {
+			return nil, ebzkratos.New(erk)
+		}
+		return nil, ebzkratos.New(pb.ErrorServerError("tx: %v", err))
 	}
-	return &res, nil
+	return &Article{
+		ID:      int64(article.ID),
+		Title:   article.Title,
+		Content: article.Content,
+	}, nil
 }
 
 func (uc *ArticleUsecase) UpdateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
 	must.True(a.ID > 0)
 	must.Nice(a.Title)
 
-	var res Article
-	if err := gofakeit.Struct(&res); err != nil {
-		return nil, ebzkratos.New(pb.ErrorServerError("fake: %v", err))
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
-	return &res, nil
+
+	return a, nil
 }
 
 func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, id int64) *ebzkratos.Ebz {
 	must.True(id > 0)
 
+	db := uc.data.DB()
+
+	// Use gormrepo DeleteW with type-safe where condition
+	if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Where(cls.ID.Eq(uint(id)))
+	}); err != nil {
+		return ebzkratos.New(pb.ErrorServerError("delete: %v", err))
+	}
 	return nil
 }
 
 func (uc *ArticleUsecase) GetArticle(ctx context.Context, id int64) (*Article, *ebzkratos.Ebz) {
 	must.True(id > 0)
 
-	var res Article
-	if err := gofakeit.Struct(&res); err != nil {
-		return nil, ebzkratos.New(pb.ErrorServerError("fake: %v", err))
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
+		}
+		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
 	}
-	return &res, nil
+
+	return &Article{
+		ID:      int64(article.ID),
+		Title:   article.Title,
+		Content: article.Content,
+	}, nil
 }
 
 func (uc *ArticleUsecase) ListArticles(ctx context.Context, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
-	var items []*Article
-	gofakeit.Slice(&items)
+	db := uc.data.DB()
+
+	// Use gormrepo Find to get all records from database
+	articles, err := uc.repo.With(ctx, db).Find(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
+		return db.Order(cls.ID.Ob("DESC").Ox())
+	})
+	if err != nil {
+		return nil, 0, ebzkratos.New(pb.ErrorServerError("list: %v", err))
+	}
+
+	items := make([]*Article, 0, len(articles))
+	for _, v := range articles {
+		items = append(items, &Article{
+			ID:      int64(v.ID),
+			Title:   v.Title,
+			Content: v.Content,
+		})
+	}
 	return items, int32(len(items)), nil
 }
```

## internal/data/data.go (+14 -3)

```diff
@@ -4,10 +4,12 @@
 	"github.com/go-kratos/kratos/v2/log"
 	"github.com/google/wire"
 	"github.com/yylego/kratos-examples/demo2kratos/internal/conf"
+	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
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
+	must.Done(db.AutoMigrate(&models.Article{}))
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

## internal/pkg/models/article.go (+13 -0)

```diff
@@ -0,0 +1,13 @@
+package models
+
+import "gorm.io/gorm"
+
+type Article struct {
+	gorm.Model
+	Title   string `gorm:"type:varchar(255)"`
+	Content string `gorm:"type:text"`
+}
+
+func (*Article) TableName() string {
+	return "articles"
+}
```

## internal/pkg/models/gormcnm.gen.go (+43 -0)

```diff
@@ -0,0 +1,43 @@
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

