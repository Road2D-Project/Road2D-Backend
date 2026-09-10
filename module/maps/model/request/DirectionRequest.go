package request

import "Road-To-Destination-BE/module/utils/enum"

type DirectionRequest struct {
	Origin       string       `form:"origin" binding:"required"`
	Destination  string       `form:"destination" binding:"required"`
	Vehicle      enum.Vehicle `form:"vehicle" swaggertype:"string" example:"bike"`
	Alternatives bool         `form:"alternatives"`
}
