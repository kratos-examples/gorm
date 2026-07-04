package models

import "gorm.io/gorm"

// Article mirrors demo2kratos's articles table. This is the student service, so
// it does not own articles — it keeps this duplicate just to cascade-delete a
// student's articles when the student is removed (the two services share one database).
//
// Article 与 demo2kratos 的 articles 表结构一致。这里是学生服务、不拥有文章表，
// 保留这份镜像仅用于删学生时顺带删掉他名下的文章（两服务共用一个库）。
type Article struct {
	gorm.Model
	Title     string `gorm:"type:varchar(255)"`
	Content   string `gorm:"type:text"`
	StudentID int64  `gorm:"index"`
}

func (*Article) TableName() string {
	return "articles"
}
