package response

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
)

type recordingImageSigner struct {
	namespace, path string
	err             error
}

func (s *recordingImageSigner) DownloadURL(_ context.Context, namespace, path string) (string, error) {
	s.namespace, s.path = namespace, path
	return "https://storage.example/view?signed=true", s.err
}

func TestTripAndGraphResolvePrivateImagesWithoutExposingKeys(t *testing.T) {
	trip := &model.Trip{}
	trip.ID = uuid.New()
	key := trip.ID.String() + "/" + uuid.NewString() + ".png"
	trip.ImagePath = &key
	for _, graph := range []bool{false, true} {
		signer := &recordingImageSigner{}
		var payload interface {
			ResolveImageURLs(context.Context, ImageURLSigner) error
		}
		if graph {
			detail := FromTripDetail(trip, nil)
			payload = &detail
		} else {
			payload = FromTripPtr(trip, nil)
		}
		if err := payload.ResolveImageURLs(context.Background(), signer); err != nil {
			t.Fatal(err)
		}
		if signer.namespace != objectStorage.TripNamespace || signer.path != key {
			t.Fatalf("signer=%+v", signer)
		}
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), key) || !strings.Contains(string(data), "signed=true") {
			t.Fatalf("unexpected response %s", data)
		}
	}
	if *trip.ImagePath != key || trip.ImageURL != nil {
		t.Fatal("response signing mutated the model")
	}
}

func TestTripListResolvesImagesAndPropagatesSigningFailure(t *testing.T) {
	trip := &model.Trip{}
	trip.ID = uuid.New()
	key := trip.ID.String() + "/" + uuid.NewString() + ".png"
	trip.ImagePath = &key
	list := &TripListResponse{Trips: []TripResponse{FromTrip(trip, nil), {}}}
	if err := list.ResolveImageURLs(context.Background(), &recordingImageSigner{}); err != nil {
		t.Fatal(err)
	}
	if list.Trips[0].ImageURL == nil || list.Trips[1].ImageURL != nil {
		t.Fatal("unexpected optional URLs")
	}
	if err := list.ResolveImageURLs(context.Background(), &recordingImageSigner{err: objectStorage.ErrNotConfigured}); !errors.Is(err, objectStorage.ErrNotConfigured) {
		t.Fatalf("error=%v", err)
	}
}

func TestTripImageRejectsAnotherTripKey(t *testing.T) {
	key := uuid.NewString() + "/" + uuid.NewString() + ".png"
	out := &TripResponse{ID: uuid.New(), ImagePath: &key}
	signer := &recordingImageSigner{}
	if err := out.ResolveImageURLs(context.Background(), signer); !errors.Is(err, objectStorage.ErrInvalidPath) {
		t.Fatal(err)
	}
	if signer.path != "" {
		t.Fatal("foreign key was signed")
	}
}
