package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/utils/enum"
	"Road-To-Destination-BE/utils/polyline"
)

type recordingDirectionClient struct {
	lastReq request.DirectionRequest
	out     *response.DirectionResponse
	err     error
}

func (s *recordingDirectionClient) Direction(_ context.Context, req request.DirectionRequest) (*response.DirectionResponse, error) {
	s.lastReq = req
	return s.out, s.err
}

func TestRedirectServiceRoutesAPointToAStopByBike(t *testing.T) {
	points := []polyline.Point{
		{Lat: 10.7769, Lng: 106.7009},
		{Lat: 10.7800, Lng: 106.7050},
	}
	stub := &recordingDirectionClient{
		out: &response.DirectionResponse{
			Routes: []response.Route{{
				Legs: []response.Leg{{
					Distance: response.TextValue{Value: 640},
					Duration: response.TextValue{Value: 95},
					Steps: []response.Step{{
						Polyline: response.OverviewPolyline{Points: polyline.Encode(points)},
					}},
				}},
				OverviewPolyline: response.OverviewPolyline{Points: "coarse"},
			}},
		},
	}
	got, distanceM, durationS, err := NewRedirectService(stub).Redirect(context.Background(), 10.7769, 106.7009, 10.78, 106.705)
	if err != nil {
		t.Fatal(err)
	}
	if got != polyline.Encode(points) || distanceM != 640 || durationS != 95 {
		t.Fatalf("polyline %q distance %.0f duration %.0f", got, distanceM, durationS)
	}
	if stub.lastReq.Vehicle != enum.BIKE || stub.lastReq.Alternatives {
		t.Fatalf("request %+v", stub.lastReq)
	}
	if stub.lastReq.Origin != "10.776900,106.700900" || stub.lastReq.Destination != "10.780000,106.705000" {
		t.Fatalf("request %+v", stub.lastReq)
	}
}

func TestRedirectServicePassesTheProviderErrorThrough(t *testing.T) {
	boom := errors.New("goong down")
	_, _, _, err := NewRedirectService(&recordingDirectionClient{err: boom}).Redirect(context.Background(), 1, 2, 3, 4)
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}

func TestRedirectServiceRejectsAnEmptyRoute(t *testing.T) {
	svc := NewRedirectService(&recordingDirectionClient{out: &response.DirectionResponse{}})
	_, _, _, err := svc.Redirect(context.Background(), 1, 2, 3, 4)
	if !errors.Is(err, ErrEmptyRedirectRoute) {
		t.Fatalf("no routes: %v", err)
	}

	svc = NewRedirectService(&recordingDirectionClient{out: &response.DirectionResponse{
		Routes: []response.Route{{Legs: []response.Leg{{Distance: response.TextValue{Value: 1}}}}},
	}})
	_, _, _, err = svc.Redirect(context.Background(), 1, 2, 3, 4)
	if !errors.Is(err, ErrEmptyRedirectRoute) {
		t.Fatalf("no polyline: %v", err)
	}
}
