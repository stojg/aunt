package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"sync"
	"time"
)

func Get(regions []*string) []*Table {
	dynamoDBs := make([]chan *Table, 0)
	for _, region := range regions {
		dynamoDBs = append(dynamoDBs, tables(region))
	}
	merged := merge(dynamoDBs)
	metrics := metrics(merged)
	filtered := filter(metrics)

	// drain channel
	list := make([]*Table, 0)
	for i := range filtered {
		list = append(list, i)
	}
	return list
}

type List struct {
	sync.RWMutex
	items []*Table
}

func NewList() *List {
	return &List{
		items: make([]*Table, 0),
	}
}

func (i *List) Get() []*Table {
	i.RLock()
	defer i.RUnlock()
	return i.items
}

func (i *List) Set(list []*Table) {
	i.Lock()
	defer i.Unlock()
	i.items = list
}

func newTable(t *dynamodb.TableDescription, region *string) *Table {
	return &Table{
		ResourceID: *t.TableName,
		LaunchTime: t.CreationDateTime,
		Name:       *t.TableName,
		Region:     *region,
		Items:      *t.ItemCount,
		namespace:  aws.String("AWS/DynamoDB"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("TableName"),
				Value: t.TableName,
			},
		},
	}
}

type Table struct {
	ResourceID          string
	LaunchTime          *time.Time
	Items               int64
	Name                string
	Region              string
	namespace           *string
	dimensions          []*cloudwatch.Dimension
	WriteThrottleEvents float64
	ReadThrottleEvents  float64
}

func (t *Table) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  t.namespace,
		MetricName: aws.String(metricName),
		Dimensions: t.dimensions,
		StartTime:  aws.Time(time.Now().Add(-1 * time.Hour)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(3600),
		Statistics: []*string{
			aws.String("SampleCount"),
		},
	}
	result, err := cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Println(err)
		return -1
	}

	if len(result.Datapoints) == 0 {
		return 0
	}
	return *result.Datapoints[0].SampleCount
}
