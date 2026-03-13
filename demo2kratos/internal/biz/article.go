package biz

import (
	"context"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/yylego/gormrepo"
	"github.com/yylego/gormrepo/gormclass"
	"github.com/yylego/kratos-ebz/ebzkratos"
	pb "github.com/yylego/kratos-examples/demo2kratos/api/article"
	"github.com/yylego/kratos-examples/demo2kratos/internal/data"
	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
	"github.com/yylego/kratos-gorm/gormkratos"
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
	var res Article
	if err := gofakeit.Struct(&res); err != nil {
		return nil, ebzkratos.New(pb.ErrorArticleCreateFailure("fake: %v", err))
	}

	db := uc.data.DB()

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
		record := &models.Article{
			Title:   res.Title,
			Content: res.Content,
		}
		if err := uc.repo.With(ctx, db).Create(record); err != nil {
			return errors.New(500, "DB_ERROR", err.Error())
		}
		return nil
	}); err != nil {
		if erk != nil {
			return nil, ebzkratos.New(erk)
		}
		return nil, ebzkratos.New(pb.ErrorServerError("tx: %v", err))
	}
	return &res, nil
}

func (uc *ArticleUsecase) UpdateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
	var res Article
	if err := gofakeit.Struct(&res); err != nil {
		return nil, ebzkratos.New(pb.ErrorServerError("fake: %v", err))
	}
	return &res, nil
}

func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, id int64) *ebzkratos.Ebz {
	return nil
}

func (uc *ArticleUsecase) GetArticle(ctx context.Context, id int64) (*Article, *ebzkratos.Ebz) {
	db := uc.data.DB()

	// Use gormrepo with type-safe column reference
	// The cls param provides compile-time safe column access
	record, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	})
	if erb != nil {
		if erb.NotExist {
			return nil, ebzkratos.New(pb.ErrorServerError("not found: %v", erb.Cause))
		}
		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
	}

	return &Article{
		ID:      int64(record.ID),
		Title:   record.Title,
		Content: record.Content,
	}, nil
}

func (uc *ArticleUsecase) ListArticles(ctx context.Context, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
	var items []*Article
	gofakeit.Slice(&items)
	return items, int32(len(items)), nil
}
