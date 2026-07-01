package data

import (
	"log/slog"

	"github.com/google/wire"
	"github.com/yylego/kratos-examples/demo2kratos/internal/conf"
	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
	"github.com/yylego/must"
	"github.com/yylego/rese"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ProviderSet = wire.NewSet(NewData)

type Data struct {
	db *gorm.DB
}

func NewData(c *conf.Data, logger *slog.Logger) (*Data, func(), error) {
	must.Same(c.Database.Driver, "postgres")
	db := rese.P1(gorm.Open(postgres.Open(c.Database.Source), &gorm.Config{}))

	must.Done(db.AutoMigrate(&models.Article{}))

	cleanup := func() {
		logger.Info("closing the data resources")
		must.Done(rese.P1(db.DB()).Close())
	}
	return &Data{db: db}, cleanup, nil
}

func (d *Data) DB() *gorm.DB {
	return d.db
}
