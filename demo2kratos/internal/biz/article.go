package biz

import (
	"context"
	"log/slog"

	"github.com/go-kratos/kratos/v3/errors"
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
	"gorm.io/gorm/clause"
)

type Article struct {
	ID        int64
	Title     string
	Content   string
	StudentID int64
}

type ArticleUsecase struct {
	data *data.Data
	repo *gormrepo.Repo[models.Article, *models.ArticleColumns]
	// The mirrored student repo backs the existence check; the two services share one database.
	// 镜像的学生 repo 用于存在性校验；两个服务共用一个库。
	repoStudent *gormrepo.Repo[models.Student, *models.StudentColumns]
	log         *slog.Logger
}

func NewArticleUsecase(data *data.Data, logger *slog.Logger) *ArticleUsecase {
	return &ArticleUsecase{
		data:        data,
		repo:        gormrepo.NewRepo(gormclass.Use(&models.Article{})),
		repoStudent: gormrepo.NewRepo(gormclass.Use(&models.Student{})),
		log:         logger,
	}
}

func (uc *ArticleUsecase) CreateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
	must.Nice(a.Title)
	must.True(a.StudentID > 0)

	db := uc.data.DB()

	// Translate the stump: FOR SHARE lock the student row inside the transaction, then insert.
	// The lock blocks a concurrent DeleteStudent (FOR UPDATE), so we never point an article at a student being deleted.
	// 翻译桩子：事务里 FOR SHARE 锁住学生行再插文章，挡住并发的 DeleteStudent，绝不指向"正被删的学生"。
	var article *models.Article
	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		if _, erb := uc.repoStudent.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
			return db.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).Where(cls.ID.Eq(uint(a.StudentID)))
		}); erb != nil {
			if erb.NotExist {
				return pb.ErrorBadParam("student %d does not exist", a.StudentID)
			}
			return pb.ErrorDbError("get student: %v", erb.Cause)
		}
		article = &models.Article{Title: a.Title, Content: a.Content, StudentID: a.StudentID}
		if err := uc.repo.With(ctx, db).Create(article); err != nil {
			return pb.ErrorArticleCreateFailure("create article: %v", err)
		}
		return nil
	}); err != nil {
		if erk != nil {
			return nil, ebzkratos.New(erk)
		}
		return nil, ebzkratos.New(pb.ErrorTxError("tx: %v", err))
	}
	return &Article{
		ID:        int64(article.ID),
		Title:     article.Title,
		Content:   article.Content,
		StudentID: article.StudentID,
	}, nil
}

func (uc *ArticleUsecase) UpdateArticle(ctx context.Context, a *Article) (*Article, *ebzkratos.Ebz) {
	must.True(a.ID > 0)
	must.Nice(a.Title)
	must.True(a.StudentID > 0)

	db := uc.data.DB()

	// Same FOR SHARE lock on the new owning student, plus confirm the article exists.
	// 与创建相同的 FOR SHARE 锁住新归属学生，再确认文章本身存在。
	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		if _, erb := uc.repoStudent.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
			return db.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).Where(cls.ID.Eq(uint(a.StudentID)))
		}); erb != nil {
			if erb.NotExist {
				return pb.ErrorBadParam("student %d does not exist", a.StudentID)
			}
			return pb.ErrorDbError("get student: %v", erb.Cause)
		}
		if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
			return db.Where(cls.ID.Eq(uint(a.ID)))
		}); erb != nil {
			if erb.NotExist {
				return pb.ErrorArticleNotFound("article %d not found", a.ID)
			}
			return pb.ErrorDbError("get article: %v", erb.Cause)
		}
		if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
			return db.Where(cls.ID.Eq(uint(a.ID)))
		}, func(cls *models.ArticleColumns) gormcnm.ColumnValueMap {
			return cls.Kw(cls.Title.Kv(a.Title)).Kw(cls.Content.Kv(a.Content)).Kw(cls.StudentID.Kv(a.StudentID))
		}); err != nil {
			return pb.ErrorDbError("update article: %v", err)
		}
		return nil
	}); err != nil {
		if erk != nil {
			return nil, ebzkratos.New(erk)
		}
		return nil, ebzkratos.New(pb.ErrorTxError("tx: %v", err))
	}
	return a, nil
}

func (uc *ArticleUsecase) DeleteArticle(ctx context.Context, id int64) *ebzkratos.Ebz {
	must.True(id > 0)

	db := uc.data.DB()

	if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	}); erb != nil {
		if erb.NotExist {
			return ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
		}
		return ebzkratos.New(pb.ErrorDbError("get article: %v", erb.Cause))
	}

	if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	}); err != nil {
		return ebzkratos.New(pb.ErrorDbError("delete article: %v", err))
	}
	return nil
}

func (uc *ArticleUsecase) GetArticle(ctx context.Context, id int64) (*Article, *ebzkratos.Ebz) {
	must.True(id > 0)

	db := uc.data.DB()

	article, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	})
	if erb != nil {
		if erb.NotExist {
			return nil, ebzkratos.New(pb.ErrorArticleNotFound("article %d not found", id))
		}
		return nil, ebzkratos.New(pb.ErrorDbError("get article: %v", erb.Cause))
	}

	return &Article{
		ID:        int64(article.ID),
		Title:     article.Title,
		Content:   article.Content,
		StudentID: article.StudentID,
	}, nil
}

func (uc *ArticleUsecase) ListArticles(ctx context.Context, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
	must.True(page >= 1)
	must.True(pageSize >= 1)

	db := uc.data.DB()

	articles, total, err := uc.repo.With(ctx, db).FindPageAndCount(
		func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
			return db
		},
		func(cls *models.ArticleColumns) gormcnm.OrderByBottle {
			return cls.ID.Ob("asc")
		},
		&gormrepo.Pagination{
			Offset: int((page - 1) * pageSize),
			Limit:  int(pageSize),
		},
	)
	if err != nil {
		return nil, 0, ebzkratos.New(pb.ErrorDbError("list articles: %v", err))
	}

	return toArticleItems(articles), int32(total), nil
}

// ListStudentArticles returns one student's articles, one page at a time. The
// student↔article relationship gets its own endpoint instead of overloading ListArticles.
//
// ListStudentArticles 分页返回某个学生的文章，关系单独开一个接口。
func (uc *ArticleUsecase) ListStudentArticles(ctx context.Context, studentID int64, page int32, pageSize int32) ([]*Article, int32, *ebzkratos.Ebz) {
	must.True(studentID > 0)
	must.True(page >= 1)
	must.True(pageSize >= 1)

	db := uc.data.DB()

	articles, total, err := uc.repo.With(ctx, db).FindPageAndCount(
		func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
			return db.Where(cls.StudentID.Eq(studentID))
		},
		func(cls *models.ArticleColumns) gormcnm.OrderByBottle {
			return cls.ID.Ob("asc")
		},
		&gormrepo.Pagination{
			Offset: int((page - 1) * pageSize),
			Limit:  int(pageSize),
		},
	)
	if err != nil {
		return nil, 0, ebzkratos.New(pb.ErrorDbError("list student articles: %v", err))
	}

	return toArticleItems(articles), int32(total), nil
}

func toArticleItems(articles []*models.Article) []*Article {
	items := make([]*Article, 0, len(articles))
	for _, v := range articles {
		items = append(items, &Article{
			ID:        int64(v.ID),
			Title:     v.Title,
			Content:   v.Content,
			StudentID: v.StudentID,
		})
	}
	return items
}
