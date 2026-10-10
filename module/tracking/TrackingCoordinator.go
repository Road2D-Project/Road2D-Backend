package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// coupling:
// realtime & trip -> tracking: sẽ giữ thông tin của trip
type TrackingCoordinator struct {
	db              *gorm.DB
	tripInformation *model.TripInformation
	progresses      map[uuid.UUID]*engine.TravelProgress
}

func NewTrackingModule(db *gorm.DB) realtime.ModuleFactory {
	return func(roomID string) realtime.Module {
		start := time.Now()
		information, err := model.LoadTripInformation(db, roomID)
		if err != nil {
			// A nil module refuses room creation. The lobby then closes the socket.
			log.Printf("tracking: load trip %s: %v", roomID, err)
			return nil
		}
		log.Printf("Tracking %s module initialized in %d ns", roomID, time.Since(start).Nanoseconds())
		return &TrackingCoordinator{
			db:              db,
			tripInformation: information,
			progresses:      make(map[uuid.UUID]*engine.TravelProgress),
		}
	}
}
