package request

type DirectionRequest struct {
	Origin       string `form:"origin" binding:"required"`
	Destination  string `form:"destination" binding:"required"`
	Vehicle      string `form:"vehicle"`
	Alternatives bool   `form:"alternatives"`
}
