package models

import "gorm.io/gorm"

type Article struct {
	gorm.Model
	Title   string `gorm:"type:varchar(255)"`
	Content string `gorm:"type:text"`
}

func (*Article) TableName() string {
	return "articles"
}
