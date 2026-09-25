package service

import (
	"context"
	"strconv"
	"sync"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// computeWorkers matches the Goong limiter burst in GoongClient.
const computeWorkers = 5

// LegRouter loads one A→B leg. *maps/service.DirectionService satisfies it.
type LegRouter interface {
	Route(ctx context.Context, req request.DirectionRequest) (*model.Leg, error)
}
type ComputeTripService struct {
	routes LegRouter
}

func NewComputeTripService(routes LegRouter) *ComputeTripService {
	return &ComputeTripService{routes: routes}
}

// define which its branch and leg
type routeSlot struct {
	branchIndex int
	legIndex    int
}

// routeJob is one unique bike leg. Slots are every branch row that needs it.
type routeJob struct {
	from  model.Destination
	to    model.Destination
	slots []routeSlot
}

type routedLeg struct {
	job routeJob
	leg *model.Leg
}

func (s *ComputeTripService) ComputeTrip(ctx context.Context, graph model.GraphBranch) (*model.TravelGraph, error) {
	result, jobs := flattenRouteJobs(graph)
	if len(jobs) == 0 {
		return &result, nil
	}
	if err := s.routeJobs(ctx, graph, result, jobs); err != nil {
		return nil, err
	}
	return &result, nil
}

// flattenRouteJobs pairs consecutive stops and collapses the same bike coordinates
// into one job. A branch shorter than two stops keeps an empty row so indexes stay aligned.
// Can be use to history version control of graph
func flattenRouteJobs(graph model.GraphBranch) (model.TravelGraph, []routeJob) {
	result := make(model.TravelGraph, len(graph))
	var jobs []routeJob
	indexByKey := make(map[string]int)
	for b, branch := range graph {
		legCount := 0
		if len(branch) >= 2 {
			legCount = len(branch) - 1
		}
		result[b] = make([]model.Travel, legCount)
		for i := 0; i < legCount; i++ {
			from := branch[i]
			to := branch[i+1]
			key := routeDedupeKey(from.Lat, from.Lng, to.Lat, to.Lng)
			slot := routeSlot{branchIndex: b, legIndex: i}
			if idx, ok := indexByKey[key]; ok {
				jobs[idx].slots = append(jobs[idx].slots, slot)
				continue
			}
			indexByKey[key] = len(jobs)
			jobs = append(jobs, routeJob{
				from:  from,
				to:    to,
				slots: []routeSlot{slot},
			})
		}
	}
	return result, jobs
}

func routeDedupeKey(fromLat, fromLng, toLat, toLng float64) string {
	return formatLatLng(fromLat, fromLng) + "|" + formatLatLng(toLat, toLng) + "|" + enum.BIKE.String()
}

// Importance
func formatLatLng(lat, lng float64) string {
	return strconv.FormatFloat(lat, 'f', -1, 64) + "," + strconv.FormatFloat(lng, 'f', -1, 64)
}

// travelFromLeg copies a computed leg onto one stop pair.
// Ids come from the slot destinations: a leg only has coordinates.
// IsFrozen stays false because freezing happens when the trip locks, not while computing.
// LegID is set only when the leg already has a database id.
func travelFromLeg(leg *model.Leg, from, to model.Destination) model.Travel {
	travel := model.Travel{
		FromDestinationID: from.ID,
		ToDestinationID:   to.ID,
		Vehicle:           leg.Vehicle,
		Polyline:          leg.Polyline,
		DistanceM:         leg.DistanceM,
		DurationS:         leg.DurationS,
		IsFrozen:          false,
		LastComputedAt:    leg.LastComputedAt,
	}
	if leg.ID != uuid.Nil {
		id := leg.ID
		travel.LegID = &id
	}
	return travel
}

func (s *ComputeTripService) routeJobs(ctx context.Context, graph model.GraphBranch, result model.TravelGraph, jobs []routeJob) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan routeJob, len(jobs))
	resultCh := make(chan routedLeg, len(jobs))
	errCh := make(chan error, 1)

	workers := computeWorkers
	if len(jobs) < workers {
		workers = len(jobs)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	// Fan out
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			s.routeWorker(ctx, jobCh, resultCh, errCh, cancel)
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()
	// fan in
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	for item := range resultCh {
		applyLeg(result, item.job, item.leg)
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

func (s *ComputeTripService) routeWorker(ctx context.Context, jobs <-chan routeJob, results chan<- routedLeg, errCh chan<- error, cancel context.CancelFunc) {
	for job := range jobs {
		// A sibling already failed and cancelled. Drain the job and do not report that cancel.
		if ctx.Err() != nil {
			continue
		}
		leg, err := s.routes.Route(ctx, request.DirectionRequest{
			Origin:       formatLatLng(job.from.Lat, job.from.Lng),
			Destination:  formatLatLng(job.to.Lat, job.to.Lng),
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
		results <- routedLeg{job: job, leg: leg}
	}
}

// v1 Đọc thông tin ngược lại graph, cũng có nguy cơ graph bị mutable -> có thể dẫn đến result sai
//
//	func applyLeg(result model.TravelGraph, graph model.GraphBranch, job routeJob, leg *model.Leg) {
//		for _, slot := range job.slots {
//			from := graph[slot.branchIndex][slot.legIndex]
//			to := graph[slot.branchIndex][slot.legIndex+1]
//			result[slot.branchIndex][slot.legIndex] = travelFromLeg(leg, from, to)
//		}
//	}
//
// v2 Không phụ thuộc vào bên ngoài
func applyLeg(result model.TravelGraph, job routeJob, leg *model.Leg) {
	for _, slot := range job.slots {
		result[slot.branchIndex][slot.legIndex] = travelFromLeg(leg, job.from, job.to)
	}
}
