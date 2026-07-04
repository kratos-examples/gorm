package models

import "gorm.io/gorm"

type Student struct {
	gorm.Model
	Name      string `gorm:"type:varchar(255)"`
	Age       int32  `gorm:"type:int"`
	ClassName string `gorm:"type:varchar(255)"`
}

func (*Student) TableName() string {
	return "students"
}
