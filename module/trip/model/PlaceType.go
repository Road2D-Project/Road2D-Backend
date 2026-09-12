package model

import "Road-To-Destination-BE/utils"

type PlaceType struct {
	utils.Base
	TypeName string `json:"typeName" gorm:"column:type_name;type:varchar(255);uniqueIndex;not null"`
}

func (PlaceType) TableName() string {
	return "place_types"
}
