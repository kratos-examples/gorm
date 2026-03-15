package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/yylego/gormcnm"
	"github.com/yylego/gormrepo"
	"github.com/yylego/gormrepo/gormclass"
	"github.com/yylego/kratos-ebz/ebzkratos"
	pb "github.com/yylego/kratos-examples/demo2kratos/api/article"
	"github.com/yylego/kratos-examples/demo2kratos/internal/data"
	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
	"github.com/yylego/kratos-gorm/gormkratos"
	"github.com/yylego/must"
	"gorm.io/gorm"
)

type Article struct {
	ID        int64
	Title     string
	Content   string
	StudentID int64
}

type ArticleUsecase struct {
	data *data.Data
	// Embed a generic repo instance to demo gormrepo usage
	// In practice, this repo can replace repetitive CRUD code
	repo *gormrepo.Repo[models.Article, *models.ArticleColumns]
	log  *log.Helper
}

func NewArticleUsecase(data *data.Data, logger log.Logger) *ArticleUsecase {
	return &ArticleUsecase{
		data: data,
		repo: gormrepo.NewRepo(gormclass.Use(&models.Article{})),
		log:  log.NewHelper(logger),
	}
}

func (uc *ArticleUsecase) CreateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
	must.Nice(a.Title)

	db := uc.data.DB()

	var article *models.Article

	// This demonstrates how to handle database transactions in Kratos framework
	//
	// IMPORTANT: Two-Errors Return Pattern
	// The gormkratos.Transaction function returns two errors:
	// - erk: Business logic errors (Kratos framework errors)
	// - err: Database transaction errors
	//
	// When erk != nil, err is always != nil (business error triggers transaction rollback).
	// So check err first as the main condition, then check erk inside.
	// When erk != nil, it contains the specific business reason.
	// Return erk first since it has more business context (reason and code) than what the raw transaction throws.
	//
	// Recommended usage pattern (MUST follow):
	//   if erk, err := gormkratos.Transaction(...); err != nil {
	//       if erk != nil {
	//           return erk  // Business error caused rollback
	//       }
	//       return WrapTxError(err)  // Database commit failed
	//   }
	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		article = &models.Article{
			Title:   a.Title,
			Content: a.Content,
		}
		if err := uc.repo.With(ctx, db).Create(article); err != nil {
			return errors.New(500, "DB_ERROR", err.Error())
		}
		return nil
	}); err != nil {
		if erk != nil {
			return nil, ebzkratos.New(erk)
		}
		return nil, ebzkratos.New(pb.ErrorServerError("tx: %v", err))
	}
	return &Article{
		ID:      int64(article.ID),
		Title:   article.Title,
		Content: article.Content,
	}, nil
}

func (uc *ArticleUsecase) UpdateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
	must.True(a.ID > 0)
	must.Nice(a.Title)

	db := uc.data.DB()

	// Use gormrepo UpdatesM with type-safe column value map
	if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(a.ID)))
	}, func(cls *models.ArticleColumns) gormcnm.ColumnValueMap {
		return cls.Kw(cls.Title.Kv(a.Title)).Kw(cls.Content.Kv(a.Content))
	}); err != nil {
		return nil, ebzkratos.New(pb.ErrorServerError("update: %v", err))
	}

	return a, nil
}

func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, id int64) *ebzkratos.Ebz {
	must.True(id > 0)

	db := uc.data.DB()

	// Use gormrepo DeleteW with type-safe where condition
	if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	}); err != nil {
		return ebzkratos.New(pb.ErrorServerError("delete: %v", err))
	}
	return nil
}

func (uc *ArticleUsecase) GetArticle(ctx context.Context, id int64) (*Article, *ebzkratos.Ebz) {
	must.True(id > 0)

	db := uc.data.DB()

	// Use gormrepo with type-safe column reference
	// The cls param provides compile-time safe column access
	article, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	})
	if erb != nil {
		if erb.NotExist {
			return nil, ebzkratos.New(pb.ErrorServerError("not found: %v", erb.Cause))
		}
		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
	}

	return &Article{
		ID:      int64(article.ID),
		Title:   article.Title,
		Content: article.Content,
	}, nil
}

func (uc *ArticleUsecase) ListArticles(ctx context.Context, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
	db := uc.data.DB()

	// Use gormrepo Find to get all records from database
	articles, err := uc.repo.With(ctx, db).Find(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Order(cls.ID.Ob("DESC").Ox())
	})
	if err != nil {
		return nil, 0, ebzkratos.New(pb.ErrorServerError("list: %v", err))
	}

	items := make([]*Article, 0, len(articles))
	for _, v := range articles {
		items = append(items, &Article{
			ID:      int64(v.ID),
			Title:   v.Title,
			Content: v.Content,
		})
	}
	return items, int32(len(items)), nil
}
