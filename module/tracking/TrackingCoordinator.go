package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/model"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	LocationMessage = "tracking.location"
)

// coupling:
// realtime & trip -> tracking: sẽ giữ thông tin của trip
type TrackingCondinator struct {
	db              *gorm.DB
	tripInformation *model.TripInformation
}

func (t TrackingCondinator) OnJoin(s realtime.Session, userID uuid.UUID) {
	log.Printf("User %s has joined trip %s", t.tripInformation.Riders[userID].Nickname, t.tripInformation.TripName)
}

func (t TrackingCondinator) OnLeave(s realtime.Session, userID uuid.UUID) {

}

func (t TrackingCondinator) OnMessage(s realtime.Session, userID uuid.UUID, msgType string, payload json.RawMessage) {

}

func (t TrackingCondinator) OnTick(s realtime.Session, now time.Time) {

}

func NewTrackingModule(db *gorm.DB) realtime.ModuleFactory {
	return func(roomID string) realtime.Module {
		start := time.Now()
		information, err := model.LoadTripInformation(db, roomID)
		if err != nil {
			return nil
		}
		log.Printf("Tracking %s module initialized in %d ns", roomID, time.Since(start).Nanoseconds())
		return TrackingCondinator{
			db:              db,
			tripInformation: information,
		}
	}
}
