package models

import "gorm.io/gorm"

type Student struct {
	gorm.Model
	Name string `gorm:"type:varchar(255)"`
}

func (*Student) TableName() string {
	return "students"
}
