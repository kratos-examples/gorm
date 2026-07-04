package biz

import (
	"context"
	"log/slog"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/yylego/gormcnm"
	"github.com/yylego/gormrepo"
	"github.com/yylego/gormrepo/gormclass"
	"github.com/yylego/kratos-ebz/ebzkratos"
	pb "github.com/yylego/kratos-examples/demo1kratos/api/student"
	"github.com/yylego/kratos-examples/demo1kratos/internal/data"
	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
	"github.com/yylego/kratos-gorm/gormkratos"
	"github.com/yylego/must"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Student struct {
	ID        int64
	Name      string
	Age       int32
	ClassName string
}

type StudentUsecase struct {
	data *data.Data
	repo *gormrepo.Repo[models.Student, *models.StudentColumns]
	// The mirrored article repo backs the cascade delete; the two services share one database.
	// 镜像的文章 repo 用于级联删除；两个服务共用一个库。
	repoArticle *gormrepo.Repo[models.Article, *models.ArticleColumns]
	log         *slog.Logger
}

func NewStudentUsecase(data *data.Data, logger *slog.Logger) *StudentUsecase {
	return &StudentUsecase{
		data:        data,
		repo:        gormrepo.NewRepo(gormclass.Use(&models.Student{})),
		repoArticle: gormrepo.NewRepo(gormclass.Use(&models.Article{})),
		log:         logger,
	}
}

func (uc *StudentUsecase) CreateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
	must.Nice(s.Name)

	db := uc.data.DB()

	var student *models.Student
	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		student = &models.Student{
			Name:      s.Name,
			Age:       s.Age,
			ClassName: s.ClassName,
		}
		if err := uc.repo.With(ctx, db).Create(student); err != nil {
			return pb.ErrorStudentCreateFailure("create student: %v", err)
		}
		return nil
	}); err != nil {
		if erk != nil {
			return nil, ebzkratos.New(erk)
		}
		return nil, ebzkratos.New(pb.ErrorTxError("tx: %v", err))
	}
	return &Student{
		ID:        int64(student.ID),
		Name:      student.Name,
		Age:       student.Age,
		ClassName: student.ClassName,
	}, nil
}

func (uc *StudentUsecase) UpdateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
	must.True(s.ID > 0)
	must.Nice(s.Name)

	db := uc.data.DB()

	// Confirm the student exists first, matching the stump: a missing row yields StudentNotFound.
	// 先确认学生存在，对齐桩子：查不到返回 StudentNotFound 而非静默成功。
	if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(s.ID)))
	}); erb != nil {
		if erb.NotExist {
			return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", s.ID))
		}
		return nil, ebzkratos.New(pb.ErrorDbError("get student: %v", erb.Cause))
	}

	if err := uc.repo.With(ctx, db).UpdatesM(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(s.ID)))
	}, func(cls *models.StudentColumns) gormcnm.ColumnValueMap {
		return cls.Kw(cls.Name.Kv(s.Name)).Kw(cls.Age.Kv(s.Age)).Kw(cls.ClassName.Kv(s.ClassName))
	}); err != nil {
		return nil, ebzkratos.New(pb.ErrorDbError("update student: %v", err))
	}

	return s, nil
}

func (uc *StudentUsecase) DeleteStudent(ctx context.Context, id int64) *ebzkratos.Ebz {
	must.True(id > 0)

	db := uc.data.DB()

	// Translate the stump's atomic cascade delete, in one transaction:
	//   ① FOR UPDATE lock the student row (a concurrent CreateArticle holds FOR SHARE, so the two serialize);
	//   ② delete the student's articles (children first);
	//   ③ delete the student (parent last).
	// 翻译桩子的原子级联删除，全在一个事务里：①FOR UPDATE 锁学生行 ②先删文章 ③再删学生。
	var notFound bool
	if erk, err := gormkratos.Transaction(ctx, db, func(db *gorm.DB) *errors.Error {
		if _, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
			return db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where(cls.ID.Eq(uint(id)))
		}); erb != nil {
			if erb.NotExist {
				notFound = true
				return nil
			}
			return pb.ErrorDbError("get student: %v", erb.Cause)
		}
		if err := uc.repoArticle.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.ArticleColumns) *gorm.DB {
			return db.Where(cls.StudentID.Eq(id))
		}); err != nil {
			return pb.ErrorDbError("delete articles: %v", err)
		}
		if err := uc.repo.With(ctx, db).DeleteW(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
			return db.Where(cls.ID.Eq(uint(id)))
		}); err != nil {
			return pb.ErrorDbError("delete student: %v", err)
		}
		return nil
	}); err != nil {
		if erk != nil {
			return ebzkratos.New(erk)
		}
		return ebzkratos.New(pb.ErrorTxError("delete student with articles: %v", err))
	}
	if notFound {
		return ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", id))
	}
	return nil
}

func (uc *StudentUsecase) GetStudent(ctx context.Context, id int64) (*Student, *ebzkratos.Ebz) {
	must.True(id > 0)

	db := uc.data.DB()

	student, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	})
	if erb != nil {
		if erb.NotExist {
			return nil, ebzkratos.New(pb.ErrorStudentNotFound("student %d not found", id))
		}
		return nil, ebzkratos.New(pb.ErrorDbError("get student: %v", erb.Cause))
	}

	return &Student{
		ID:        int64(student.ID),
		Name:      student.Name,
		Age:       student.Age,
		ClassName: student.ClassName,
	}, nil
}

func (uc *StudentUsecase) ListStudents(ctx context.Context, page int32, pageSize int32) ([]*Student, int32, *ebzkratos.Ebz) {
	must.True(page >= 1)
	must.True(pageSize >= 1)

	db := uc.data.DB()

	// gormrepo FindPageAndCount returns the page and the row count in one shot,
	// replacing the stump's hand-written Count + order + offset + limit.
	// gormrepo 的 FindPageAndCount 一次拿到当页数据和总行数。
	students, total, err := uc.repo.With(ctx, db).FindPageAndCount(
		func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
			return db
		},
		func(cls *models.StudentColumns) gormcnm.OrderByBottle {
			return cls.ID.Ob("asc")
		},
		&gormrepo.Pagination{
			Offset: int((page - 1) * pageSize),
			Limit:  int(pageSize),
		},
	)
	if err != nil {
		return nil, 0, ebzkratos.New(pb.ErrorDbError("list students: %v", err))
	}

	items := make([]*Student, 0, len(students))
	for _, v := range students {
		items = append(items, &Student{
			ID:        int64(v.ID),
			Name:      v.Name,
			Age:       v.Age,
			ClassName: v.ClassName,
		})
	}
	return items, int32(total), nil
}
