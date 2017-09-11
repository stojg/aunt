package stats

import "time"

// Linear solves y = mx + b by directly calculating the fitting line, ie no gradient descent

type Linear struct {
	points []Point
	m      float64 // slope
	b      float64 //intercept
}

func (l *Linear) Slope() float64 {
	return l.m
}

func (l *Linear) Intercept() float64 {
	return l.b
}

func (l *Linear) LastX() float64 {
	return l.points[len(l.points)-1].y
}

func (l *Linear) LastY() float64 {
	return l.points[len(l.points)-1].y
}

func (l *Linear) AtX(t time.Time) float64 {
	unix := float64(t.Unix())
	return (l.m * unix) + l.b
}

func (l *Linear) AtY(y float64) time.Time {
	unix := (y - l.b) / l.m
	if unix < 0 {
		return time.Time{}
	}
	return time.Unix(int64(unix), 0)
}

func NewLinear(points []Point) *Linear {

	n := float64(len(points))

	linear := &Linear{
		points: points,
	}

	var xSum float64
	var ySum float64

	for _, point := range points {
		xSum += point.x
		ySum += point.y
	}

	var xxSum float64
	var xySum float64

	for _, point := range points {
		xxSum += point.x * point.x
		xySum += point.x * point.y
	}

	// calculate slope
	linear.m = ((n * xySum) - (xSum * ySum)) / ((n * xxSum) - (xSum * xSum))

	// calculate intercept
	linear.b = (ySum - (linear.m * xSum)) / n

	return linear
}
