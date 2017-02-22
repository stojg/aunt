package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"time"
)

func Get(regions []*string) *List {
	dynamoDBs := make([]chan *table, 0)
	for _, region := range regions {
		dynamoDBs = append(dynamoDBs, fetchTables(region))
	}
	merged := merge(dynamoDBs)
	metrics := metrics(merged)
	filtered := filter(metrics)

	// drain channel
	list := NewList()
	for i := range filtered {
		list.items = append(list.items, i)
	}
	return list
}

func newTable(t *dynamodb.TableDescription, region *string) *table {
	return &table{
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

type table struct {
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

func (t *table) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
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
