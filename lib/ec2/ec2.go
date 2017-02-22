package ec2

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"strings"
	"time"
)

func Get(regions []*string) *List {
	instances := make([]chan *instance, 0)
	for _, region := range regions {
		instances = append(instances, fetchInstances(region))
	}
	merged := merge(instances)
	metrics := metrics(merged)
	filtered := filter(metrics, 10.0)

	// drain channel
	list := NewList()
	for i := range filtered {
		list.items = append(list.items, i)
	}
	return list
}

func newInstance(i *ec2.Instance, region *string) *instance {
	r := &instance{
		ResourceID:   *i.InstanceId,
		Region:       *region,
		InstanceType: *i.InstanceType,
		State:        *i.State.Name,
		LaunchTime:   i.LaunchTime,
		namespace:    aws.String("AWS/EC2"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: i.InstanceId,
			},
		},
	}
	if strings.HasPrefix(r.InstanceType, "t2") {
		r.Burstable = true
	}

	for _, tag := range i.Tags {
		if *tag.Key == "Name" && len(*tag.Value) > 0 {
			r.Name = *tag.Value
			break
		}
	}
	return r
}

type instance struct {
	ResourceID       string
	LaunchTime       *time.Time
	Name             string
	InstanceType     string
	Region           string
	State            string
	Burstable        bool
	CPUUtilization   float64
	CPUCreditBalance float64
	namespace        *string
	dimensions       []*cloudwatch.Dimension
}

func (i *instance) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  i.namespace,
		MetricName: aws.String(metricName),
		Dimensions: i.dimensions,
		StartTime:  aws.Time(time.Now().Add(-15 * time.Minute)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(3600),
		Statistics: []*string{
			aws.String("Average"),
		},
	}
	result, err := cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Println(err)
		return -1
	}
	if len(result.Datapoints) == 0 {
		return -1
	}
	return *result.Datapoints[0].Average
}
