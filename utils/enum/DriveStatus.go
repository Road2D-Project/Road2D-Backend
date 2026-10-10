package enum

//go:generate go tool enumer -type=DriveStatus -json -text -sql

type DriveStatus int

const (
	OnROUTE DriveStatus = iota // đồng nghĩa với đang online và tiếp tục chạy
	DEVIATED
	STOP // velocity = 0
	EMERGENCY
)
