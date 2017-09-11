package stats

import (
	"sort"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

type Point struct {
	x float64
	y float64
}

type sortedCloudwatch []*cloudwatch.Datapoint

func (s sortedCloudwatch) Len() int           { return len(s) }
func (s sortedCloudwatch) Less(i, j int) bool { return s[i].Timestamp.Before(*s[j].Timestamp) }
func (s sortedCloudwatch) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func Convert(metrics []*cloudwatch.Datapoint) []Point {
	sort.Sort(sortedCloudwatch(metrics))
	var points []Point
	for _, dataPoint := range metrics {
		points = append(points, Point{
			x: float64(dataPoint.Timestamp.Unix()),
			y: *dataPoint.Average,
		})
	}

	return points
}
