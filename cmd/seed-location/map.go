package main

import (
	mapResponse "Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils"
)

func detailToLocation(detailResponse mapResponse.PlaceDetailResponse) *model.Location {
	detail := detailResponse.Result
	if detail.PlaceID == "" || detail.Name == "" || detail.Geometry == nil {
		return nil
	}
	compound := model.Compound{}
	if detail.Compound != nil {
		compound = model.Compound{
			Commune:  detail.Compound.Commune,
			District: detail.Compound.District,
			Province: detail.Compound.Province,
		}
	}
	address := compound.ToAddress()
	formatted := detail.FormattedAddress
	placeID := detail.PlaceID
	return &model.Location{
		Base:             utils.Base{},
		Name:             detail.Name,
		Lat:              detail.Geometry.Location.Lat,
		Lng:              detail.Geometry.Location.Lng,
		Address:          &address,
		FormattedAddress: &formatted,
		PlaceID:          &placeID,
		Compound:         &compound,
		IsVerified:       true,
	}
}
