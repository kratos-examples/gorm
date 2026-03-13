package data

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/yylego/kratos-examples/demo2kratos/internal/conf"
	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
	"github.com/yylego/must"
	"github.com/yylego/rese"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	loggergorm "gorm.io/gorm/logger"
)

var ProviderSet = wire.NewSet(NewData)

type Data struct {
	db *gorm.DB
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	dsn := must.Nice(c.Database.Source)
	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: loggergorm.Default.LogMode(loggergorm.Info),
	}))

	must.Done(db.AutoMigrate(&models.Article{}))

	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
		must.Done(rese.P1(db.DB()).Close())
	}
	return &Data{db: db}, cleanup, nil
}

func (d *Data) DB() *gorm.DB {
	return d.db
}
