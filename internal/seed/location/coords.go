package location

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Coord struct {
	Lat float64
	Lng float64
}

func DefaultCoords() []Coord {
	return []Coord{
		{Lat: 10.7725, Lng: 106.6980},
		{Lat: 10.8721512, Lng: 106.803008},
	}
}

func ParseCoordTokens(tokens ...string) ([]Coord, error) {
	nums := make([]float64, 0)
	for _, token := range tokens {
		for _, piece := range strings.FieldsFunc(token, isCoordSep) {
			n, err := strconv.ParseFloat(piece, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %q", piece)
			}
			nums = append(nums, n)
		}
	}
	if len(nums) == 0 {
		return nil, nil
	}
	if len(nums)%2 != 0 {
		return nil, fmt.Errorf("odd number of values, want lat,lng pairs, got %v", nums)
	}
	out := make([]Coord, 0, len(nums)/2)
	for i := 0; i < len(nums); i += 2 {
		lat, lng := nums[i], nums[i+1]
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return nil, fmt.Errorf("coord out of range: %g,%g", lat, lng)
		}
		out = append(out, Coord{Lat: lat, Lng: lng})
	}
	return out, nil
}

func isCoordSep(r rune) bool {
	return r == ',' || r == ';' || unicode.IsSpace(r)
}
