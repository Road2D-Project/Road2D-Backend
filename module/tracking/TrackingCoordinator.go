package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// coupling:
// realtime & trip -> tracking: sẽ giữ thông tin của trip
type TrackingCoordinator struct {
	db                 *gorm.DB
	router             RedirectRouter
	tripInformation    *model.TripInformation
	progresses         map[uuid.UUID]*engine.TravelProgress
	recovery           RecoveryDependencies
	redirectResults    <-chan model.RedirectResult
	unsubscribe        func()
	revision           uint64
	checkpointSequence uint64
	checkpointDirty    bool
	lastCheckpointAt   time.Time
}

type CheckpointLoader interface {
	Load(context.Context, uuid.UUID) (*model.TripCheckpoint, error)
}

type CheckpointSink interface {
	Offer(model.TripCheckpoint)
}

type RedirectSubmitter interface {
	TrySubmit(model.RedirectJob) bool
}

type RedirectResultSource interface {
	Subscribe(uuid.UUID) (<-chan model.RedirectResult, func())
}

type RecoveryDependencies struct {
	Checkpoints        CheckpointLoader
	Writer             CheckpointSink
	Redirects          RedirectSubmitter
	Results            RedirectResultSource
	CheckpointInterval time.Duration
}

// NewTrackingModule builds one coordinator per room. router computes the
// temporary route when a rider leaves the branch polyline; nil disables
// redirection and every deviation reports providerError.
func NewTrackingModule(db *gorm.DB, router RedirectRouter) realtime.ModuleFactory {
	return newTrackingFactory(db, router, RecoveryDependencies{})
}

func NewTrackingModuleWithRecovery(db *gorm.DB, recovery RecoveryDependencies) realtime.ModuleFactory {
	return newTrackingFactory(db, nil, recovery)
}

func newTrackingFactory(db *gorm.DB, router RedirectRouter, recovery RecoveryDependencies) realtime.ModuleFactory {
	return func(roomID string) (realtime.Module, error) {
		start := time.Now()
		information, err := model.LoadTripInformation(db, roomID)
		if err != nil {
			log.Printf("tracking: load trip %s: %v", roomID, err)
			return nil, err
		}
		log.Printf("Tracking %s module initialized in %d ns", roomID, time.Since(start).Nanoseconds())
		coordinator := &TrackingCoordinator{
			db:              db,
			router:          router,
			tripInformation: information,
			progresses:      make(map[uuid.UUID]*engine.TravelProgress),
			recovery:        recovery,
		}
		if recovery.Checkpoints != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			checkpoint, err := recovery.Checkpoints.Load(ctx, information.TripId)
			cancel()
			if err != nil {
				return nil, err
			}
			if checkpoint != nil {
				coordinator.restoreCheckpoint(*checkpoint)
			}
		}
		return coordinator, nil
	}
}
