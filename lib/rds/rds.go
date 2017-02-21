package rds

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"strings"
	"sync"
	"time"
)

func Get(regions []*string) []*Resource {
	dynamoDBs := make([]chan *Resource, 0)
	for _, region := range regions {
		dynamoDBs = append(dynamoDBs, dbs(region))
	}
	merged := merge(dynamoDBs)
	metrics := metrics(merged)
	filtered := lowCreditFilter(metrics, 20)

	// drain channel
	list := make([]*Resource, 0)
	for i := range filtered {
		list = append(list, i)
	}
	return list
}

type List struct {
	sync.RWMutex
	items []*Resource
}

func NewList() *List {
	return &List{
		items: make([]*Resource, 0),
	}
}

func (i *List) Get() []*Resource {
	i.RLock()
	defer i.RUnlock()
	return i.items
}

func (i *List) Set(list []*Resource) {
	i.Lock()
	defer i.Unlock()
	i.items = list
}

func NewRDS(db *rds.DBInstance, region *string) *Resource {
	r := &Resource{
		ResourceID:   *db.DBInstanceIdentifier,
		Region:       *region,
		InstanceType: *db.DBInstanceClass,
		State:        *db.DBInstanceStatus,
		LaunchTime:   db.InstanceCreateTime,
		Name:         strings.Replace(*db.DBInstanceIdentifier, "-", ".", -1) + ".db",
		namespace:    aws.String("AWS/RDS"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("DBInstanceIdentifier"),
				Value: db.DBInstanceIdentifier,
			},
		},
	}
	if strings.Contains(*db.DBInstanceClass, "t2") {
		r.Burstable = true
	}
	return r
}

type Resource struct {
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

func (i *Resource) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
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
