package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/polyline"
)

// polylineFromSteps builds the leg line from turn steps. Each step polyline
// is encoded from (0, 0), so Join decodes them before stitching. A step that
// does not decode leaves the overview line in place.
func polylineFromSteps(steps []model.RouteStep, overview string) string {
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		if step.Polyline != "" {
			parts = append(parts, step.Polyline)
		}
	}
	joined, err := polyline.Join(parts)
	if err != nil || joined == "" {
		return overview
	}
	return joined
}
