package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestCreateBranch(t *testing.T) {
	d := []model.Destination{
		{
			Lat:          10.776889,
			Lng:          106.700806,
			Name:         "Bún Chả Hà Nội - Chợ Bến Thành",
			StayOverTime: 45,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7721,
			Lng:          106.6983,
			Name:         "Cà phê Trứng Dinh Độc Lập",
			StayOverTime: 60,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7798,
			Lng:          106.6990,
			Name:         "Nhà Thờ Đức Bà Sài Gòn",
			StayOverTime: 30,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7756,
			Lng:          106.7019,
			Name:         "Phố Đi Bộ Nguyễn Huệ",
			StayOverTime: 90,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7716,
			Lng:          106.7044,
			Name:         "Bến Bạch Đằng & Xe Buýt Đường Thủy",
			StayOverTime: 120,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7731,
			Lng:          106.7031,
			Name:         "Tòa nhà Bitexco Financial Tower",
			StayOverTime: 60,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7875,
			Lng:          106.7053,
			Name:         "Thảo Cầm Viên Sài Gòn",
			StayOverTime: 180,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7628,
			Lng:          106.6825,
			Name:         "Hủ Tiếu Nam Vang Vang Lừng",
			StayOverTime: 40,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7551,
			Lng:          106.6611,
			Name:         "Chùa Bà Thiên Hậu - Chợ Lớn",
			StayOverTime: 45,
			Status:       enum.Editing,
		},
		{
			Lat:          10.7932,
			Lng:          106.6913,
			Name:         "Cơm Tấm Ba Ghiền",
			StayOverTime: 50,
			Status:       enum.Editing,
		},
		{
			Lat:          11.1111,
			Lng:          111.111,
			Name:         "Cơm Tấm Ba Ghiền 2",
			StayOverTime: 50,
			Status:       enum.Editing,
		},
	}
	trip := model.Trip{
		Base:          utils.Base{},
		GroupID:       nil,
		Group:         nil,
		OwnerID:       uuid.UUID{},
		Owner:         nil,
		Name:          "",
		Status:        0,
		StartTime:     nil,
		EndTime:       nil,
		TotalDistance: 0,
		Note:          "",
		InviteToken:   "",
		Branches:      nil,
		Members:       nil,
	}
	for i := range d {
		d[i].ID = uuid.New()
	}
	//giả sử chỉ biết các phần tử sắp xếp theo order idx
	// thứ tự các branch ko cho biết branch đóng vai trò gì
	graphDes := [][]model.Destination{
		{
			d[0], d[1], d[2], d[4], d[5], d[9],
		},
		{
			// split từ d1 -> merge xuống d9
			d[1], d[10],
		},
		{
			d[0], d[4], d[6], d[7], d[8], d[9],
		},
	}
	// Tự động nối node cơ bản
	tripBranch, err := model.BuildTripBranches(&trip, graphDes)
	if err != nil {
		t.Fatal(err)
	}
	//-----
	val, err := json.Marshal(tripBranch)
	var pretty bytes.Buffer
	err = json.Indent(&pretty, val, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(pretty.Bytes()))
}
func TestMapping(t *testing.T) {
	dic := make(map[string]int)
	t.Log(dic["hello"])
}
