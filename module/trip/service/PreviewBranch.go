package service

import (
	"context"
	"errors"
	"sync"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/trip/model"
	triprequest "Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// ErrInvalidComputePoint means a stop named neither a destination nor a location, or both.
var ErrInvalidComputePoint = errors.New("each compute point needs exactly one of destinationId or locationId")

// previewStop is one resolved end of a preview hop. The id that was not asked for stays nil.
type previewStop struct {
	name          string
	lat           float64
	lng           float64
	destinationID *uuid.UUID
	locationID    *uuid.UUID
}

type previewJob struct {
	index int
	from  previewStop
	to    previewStop
}

type previewResult struct {
	index int
	leg   *model.Leg
}

// PreviewBranch routes one ordered branch and returns the hops.
// It never reads or writes travels: the caller only wanted a look at the route.
func (s *ComputeTripService) PreviewBranch(ctx context.Context, points []triprequest.ComputeBranchPoint) (*response.ComputeBranchResponse, error) {
	if len(points) < 2 {
		return nil, ErrInvalidComputePoint
	}
	stops := make([]previewStop, len(points))
	for i, point := range points {
		stop, err := s.resolvePreviewStop(ctx, point)
		if err != nil {
			return nil, err
		}
		stops[i] = stop
	}
	legs := make([]model.Leg, len(stops)-1)
	if err := s.previewRoutes(ctx, stops, legs); err != nil {
		return nil, err
	}
	return previewResponse(stops, legs), nil
}

func (s *ComputeTripService) resolvePreviewStop(ctx context.Context, point triprequest.ComputeBranchPoint) (previewStop, error) {
	hasDestination := point.DestinationID != nil && *point.DestinationID != uuid.Nil
	hasLocation := point.LocationID != nil && *point.LocationID != uuid.Nil
	if hasDestination == hasLocation {
		return previewStop{}, ErrInvalidComputePoint
	}
	if hasDestination {
		destination, err := s.destinations.FindDestinationById(ctx, *point.DestinationID)
		if err != nil {
			return previewStop{}, err
		}
		if destination == nil {
			return previewStop{}, ErrInvalidComputePoint
		}
		id := destination.ID
		return previewStop{
			name:          destination.Name,
			lat:           destination.Lat,
			lng:           destination.Lng,
			destinationID: &id,
		}, nil
	}
	location, err := s.locations.FindLocationById(ctx, *point.LocationID)
	if err != nil {
		return previewStop{}, err
	}
	if location == nil {
		return previewStop{}, ErrInvalidComputePoint
	}
	id := location.ID
	return previewStop{
		name:       location.Name,
		lat:        location.Lat,
		lng:        location.Lng,
		locationID: &id,
	}, nil
}

func (s *ComputeTripService) previewRoutes(ctx context.Context, stops []previewStop, legs []model.Leg) error {
	jobs := make([]previewJob, len(stops)-1)
	for i := 0; i < len(jobs); i++ {
		jobs[i] = previewJob{index: i, from: stops[i], to: stops[i+1]}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan previewJob, len(jobs))
	resultCh := make(chan previewResult, len(jobs))
	errCh := make(chan error, 1)

	workers := computeWorkers
	if len(jobs) < workers {
		workers = len(jobs)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			s.previewWorker(ctx, jobCh, resultCh, errCh, cancel)
		}()
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	for item := range resultCh {
		legs[item.index] = *item.leg
	}
	select {
	case err := <-errCh:
		return err
	default:
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (s *ComputeTripService) previewWorker(ctx context.Context, jobs <-chan previewJob, results chan<- previewResult, errCh chan<- error, cancel context.CancelFunc) {
	for job := range jobs {
		if ctx.Err() != nil {
			continue
		}
		leg, err := s.routes.Route(ctx, request.DirectionRequest{
			Origin:       formatLatLng(job.from.lat, job.from.lng),
			Destination:  formatLatLng(job.to.lat, job.to.lng),
			Vehicle:      enum.BIKE,
			Alternatives: false,
		})
		if err != nil {
			select {
			case errCh <- err:
				cancel()
			default:
			}
			continue
		}
		results <- previewResult{index: job.index, leg: leg}
	}
}

func previewResponse(stops []previewStop, legs []model.Leg) *response.ComputeBranchResponse {
	out := &response.ComputeBranchResponse{Legs: make([]response.ComputeBranchLeg, len(legs))}
	for i, leg := range legs {
		out.Legs[i] = response.ComputeBranchLeg{
			From:      previewStopResponse(stops[i]),
			To:        previewStopResponse(stops[i+1]),
			Vehicle:   leg.Vehicle,
			Polyline:  leg.Polyline,
			DistanceM: leg.DistanceM,
			DurationS: leg.DurationS,
		}
	}
	return out
}

func previewStopResponse(stop previewStop) response.ComputeBranchStop {
	return response.ComputeBranchStop{
		Name:          stop.name,
		DestinationID: stop.destinationID,
		LocationID:    stop.locationID,
	}
}
