package biz

import (
	"context"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/yylego/gormrepo"
	"github.com/yylego/gormrepo/gormclass"
	"github.com/yylego/kratos-ebz/ebzkratos"
	pb "github.com/yylego/kratos-examples/demo1kratos/api/student"
	"github.com/yylego/kratos-examples/demo1kratos/internal/data"
	"github.com/yylego/kratos-examples/demo1kratos/internal/pkg/models"
	"github.com/yylego/kratos-gorm/gormkratos"
	"github.com/yylego/must"
	"gorm.io/gorm"
)

type Student struct {
	ID        int64
	Name      string
	Age       int32
	ClassName string
}

type StudentUsecase struct {
	data *data.Data
	// Embed a generic repo instance to demo gormrepo usage
	// In practice, this repo can replace repetitive CRUD code
	repo *gormrepo.Repo[models.Student, *models.StudentColumns]
	log  *log.Helper
}

func NewStudentUsecase(data *data.Data, logger log.Logger) *StudentUsecase {
	return &StudentUsecase{
		data: data,
		repo: gormrepo.NewRepo(gormclass.Use(&models.Student{})),
		log:  log.NewHelper(logger),
	}
}

func (uc *StudentUsecase) CreateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
	must.Nice(s.Name)

	var res Student
	if err := gofakeit.Struct(&res); err != nil {
		return nil, ebzkratos.New(pb.ErrorStudentCreateFailure("fake: %v", err))
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
		record := &models.Student{
			Name: res.Name,
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

func (uc *StudentUsecase) UpdateStudent(ctx context.Context, s *Student) (*Student, *ebzkratos.Ebz) {
	must.True(s.ID > 0)
	must.Nice(s.Name)

	var res Student
	if err := gofakeit.Struct(&res); err != nil {
		return nil, ebzkratos.New(pb.ErrorServerError("fake: %v", err))
	}
	return &res, nil
}

func (uc *StudentUsecase) DeleteStudent(ctx context.Context, id int64) *ebzkratos.Ebz {
	must.True(id > 0)

	return nil
}

func (uc *StudentUsecase) GetStudent(ctx context.Context, id int64) (*Student, *ebzkratos.Ebz) {
	must.True(id > 0)

	db := uc.data.DB()

	// Use gormrepo with type-safe column reference
	// The cls param provides compile-time safe column access
	record, erb := uc.repo.With(ctx, db).FirstE(func(db *gorm.DB, cls *models.StudentColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(uint(id)))
	})
	if erb != nil {
		if erb.NotExist {
			return nil, ebzkratos.New(pb.ErrorServerError("not found: %v", erb.Cause))
		}
		return nil, ebzkratos.New(pb.ErrorServerError("db: %v", erb.Cause))
	}

	return &Student{
		ID:   int64(record.ID),
		Name: record.Name,
	}, nil
}

func (uc *StudentUsecase) ListStudents(ctx context.Context, page int32, pageSize int32) ([]*Student, int32, *ebzkratos.Ebz) {
	var items []*Student
	gofakeit.Slice(&items)
	return items, int32(len(items)), nil
}
