package models

import "gorm.io/gorm"

// Student mirrors demo1kratos's students table. This is the article service, so
// it does not own students — it keeps this duplicate just to check a student
// exists before creating an article (the two services share one database).
//
// Student 与 demo1kratos 的 students 表结构一致。这里是文章服务、不拥有学生表，
// 保留这份镜像仅用于建文章前校验学生存在（两服务共用一个库）。
type Student struct {
	gorm.Model
	Name      string `gorm:"type:varchar(255)"`
	Age       int32  `gorm:"type:int"`
	ClassName string `gorm:"type:varchar(255)"`
}

func (*Student) TableName() string {
	return "students"
}
