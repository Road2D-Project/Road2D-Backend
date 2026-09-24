package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

type stubGeocodeClient struct {
	calls int
	last  request.GeocodeRequest
	out   *response.GeocodeResponse
}

func (s *stubGeocodeClient) Geocode(_ context.Context, req request.GeocodeRequest) (*response.GeocodeResponse, error) {
	s.calls++
	s.last = req
	return s.out, nil
}

func TestGeocodeServiceForward(t *testing.T) {
	stub := &stubGeocodeClient{
		out: &response.GeocodeResponse{
			Status: "OK",
			Results: []response.GeocodeResult{{
				FormattedAddress: "91 Trung Kính, Yên Hòa, Hà Nội",
				PlaceID:          "place-1",
				Geometry: &response.GeocodeGeometry{
					Location: response.LatLng{Lat: 21.01367, Lng: 105.79825},
				},
				Compound: &response.AdministrativeCompound{Commune: "Yên Hòa", Province: "Hà Nội"},
				Types:    []string{"house_number"},
			}},
		},
	}
	out, err := NewGeocodeService(stub).Lookup(context.Background(), request.GeocodeRequest{
		Address: "91 Trung Kính, Trung Hòa, Cầu Giấy, Hà Nội",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 || stub.last.AddressValue() == "" {
		t.Fatalf("expected forward geocode, last=%+v", stub.last)
	}
	if out.Results[0].Geometry.Location.Lat == 0 {
		t.Fatalf("missing location: %+v", out.Results[0])
	}
}

func TestGeocodeServiceReversePassesLimitAndVNID(t *testing.T) {
	stub := &stubGeocodeClient{out: &response.GeocodeResponse{Status: "OK"}}
	_, err := NewGeocodeService(stub).Lookup(context.Background(), request.GeocodeRequest{
		LatLng:                          "15.765075, 108.204474",
		Limit:                           5,
		HasVNID:                         true,
		HasDeprecatedAdministrativeUnit: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.last.LatLngValue() != "15.765075,108.204474" {
		t.Fatalf("latlng compacted: %q", stub.last.LatLngValue())
	}
	if stub.last.Limit != 5 || !stub.last.HasVNID {
		t.Fatalf("reverse extras: %+v", stub.last)
	}
}

func TestGeocodeServiceRejectsEmptyAndMixedLookup(t *testing.T) {
	svc := NewGeocodeService(&stubGeocodeClient{})
	_, err := svc.Lookup(context.Background(), request.GeocodeRequest{})
	if !errors.Is(err, ErrGeocodeLookup) {
		t.Fatalf("empty: %v", err)
	}
	_, err = svc.Lookup(context.Background(), request.GeocodeRequest{
		Address: "91 Trung Kính",
		LatLng:  "21.01,105.79",
	})
	if !errors.Is(err, ErrGeocodeLookup) {
		t.Fatalf("mixed: %v", err)
	}
}
