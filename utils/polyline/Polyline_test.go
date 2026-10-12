package polyline

import "testing"

func TestEncodeMatchesTheKnownPolyline(t *testing.T) {
	points := []Point{
		{Lat: 38.5, Lng: -120.2},
		{Lat: 40.7, Lng: -120.95},
		{Lat: 43.252, Lng: -126.453},
	}
	const want = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	if got := Encode(points); got != want {
		t.Fatalf("encode %q", got)
	}
	decoded, err := Decode(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(points) {
		t.Fatalf("decoded %d points", len(decoded))
	}
	for i, point := range points {
		if !sameGrid(point, decoded[i]) {
			t.Fatalf("point %d: got %+v", i, decoded[i])
		}
	}
}

func TestJoinDropsTheSharedEndpoint(t *testing.T) {
	points := []Point{
		{Lat: 10.7769, Lng: 106.7009},
		{Lat: 10.7800, Lng: 106.7050},
		{Lat: 10.7900, Lng: 106.7100},
	}
	first := Encode(points[:2])
	second := Encode(points[1:])
	joined, err := Join([]string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if joined != Encode(points) {
		t.Fatalf("joined %q want %q", joined, Encode(points))
	}
}

func TestJoinRejectsABrokenChunk(t *testing.T) {
	if _, err := Join([]string{"abc"}); err == nil {
		t.Fatal("expected an invalid polyline to fail")
	}
}
