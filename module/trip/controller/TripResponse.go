package controller

import (
	"context"

	"Road-To-Destination-BE/module/trip/model/response"

	"github.com/gin-gonic/gin"
)

type tripImageResponse interface {
	ResolveImageURLs(context.Context, response.ImageURLSigner) error
}

func (ctrl *TripController) writeTripResponse(c *gin.Context, status int, body tripImageResponse) {
	if err := body.ResolveImageURLs(c.Request.Context(), ctrl.storage); err != nil {
		mapImageError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(status, body)
}
