package ec2

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"strings"
	"sync"
	"time"
)

func Get(regions []*string) []*Instance {
	instances := make([]chan *Instance, 0)
	for _, region := range regions {
		instances = append(instances, getResources(region))
	}
	merged := merge(instances)
	metrics := metrics(merged)
	lowCredits := lowCreditFilter(metrics, 10.0)

	// drain channel
	list := make([]*Instance, 0)
	for i := range lowCredits {
		list = append(list, i)
	}
	return list
}

type List struct {
	sync.RWMutex
	items []*Instance
}

func NewList() *List {
	return &List{
		items: make([]*Instance, 0),
	}
}

func (i *List) Get() []*Instance {
	i.RLock()
	defer i.RUnlock()
	return i.items
}

func (i *List) Set(list []*Instance) {
	i.Lock()
	defer i.Unlock()
	i.items = list
}

func newInstance(i *ec2.Instance, region *string) *Instance {
	r := &Instance{
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

type Instance struct {
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

func (i *Instance) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
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
