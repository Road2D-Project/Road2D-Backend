package enum

//go:generate go tool enumer -type=DriveStatus -json -text -sql

type DriveStatus int

const (
	OFFLINE DriveStatus = iota
	OnROUTE             // đồng nghĩa với đang online và tiếp tục chạy
	DEVIATED
	EMERGENCY
)
