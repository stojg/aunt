package dynamodb

import (
	"fmt"
	"time"

	"github.com/ararog/timeago"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stojg/aunt/lib/core"
)

// Headers returns a list of headers for use in a table header
func Headers() []string {
	return []string{"Table", "Read Capacity", "Sum Throttled Reads (24hr)", "Write Capacity", "Sum Throttled Writes(24hr)", "Entries", "Launched", "Region"}
}

// Fetch returns a channel where it will asynchronously will send Resources
func Fetch(region, account, roleARN string) chan core.Resource {
	out := make(chan core.Resource, 16)
	go func() {
		sess, config := core.NewConfig(region, roleARN)
		svc := dynamodb.New(sess, config)
		resp, err := svc.ListTables(nil)
		if err != nil {
			fmt.Printf("dynamodb.Fetch.ListTables - %s - %s %v\n", region, roleARN, err)
			return
		}
		for _, t := range resp.TableNames {
			tableDesc, err := svc.DescribeTable(&dynamodb.DescribeTableInput{TableName: t})
			if err != nil {
				fmt.Printf("dynamodb.FetchTables.DescribeTable - %s - %s %v\n", region, roleARN, err)
				continue
			}
			out <- newTable(tableDesc.Table, sess, config)
		}
		close(out)
	}()
	return out
}

func newTable(t *dynamodb.TableDescription, sess client.ConfigProvider, config *aws.Config) core.Resource {
	cw := cloudwatch.New(sess, config)
	return &table{
		Name:          *t.TableName,
		ResourceID:    *t.TableName,
		LaunchTime:    t.CreationDateTime,
		Entries:       *t.ItemCount,
		Region:        *config.Region,
		WriteCapacity: *t.ProvisionedThroughput.WriteCapacityUnits,
		ReadCapacity:  *t.ProvisionedThroughput.ReadCapacityUnits,
		cw:            cw,
	}
}

type table struct {
	Name                string
	ResourceID          string
	LaunchTime          *time.Time
	Region              string
	Entries             int64
	WriteThrottleEvents float64
	WriteCapacity       int64
	ReadThrottleEvents  float64
	ReadCapacity        int64

	cw *cloudwatch.CloudWatch
}

func (t *table) Row() []string {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *t.LaunchTime)
	return []string{
		t.Name,
		fmt.Sprintf("%d", t.ReadCapacity),
		fmt.Sprintf("%.0f", t.ReadThrottleEvents),
		fmt.Sprintf("%d", t.WriteCapacity),
		fmt.Sprintf("%.0f", t.WriteThrottleEvents),
		fmt.Sprintf("%d", t.Entries),
		launchedAgo,
		t.Region,
	}
}

func (t *table) Display() bool {
	return t.ReadThrottleEvents > 0 || t.WriteThrottleEvents > 0 || t.Entries > 3600
}

func (t *table) SetMetrics() {
	t.WriteThrottleEvents = t.getMetric("WriteThrottleEvents")
	t.ReadThrottleEvents = t.getMetric("ReadThrottleEvents")
}

func (t *table) getMetric(metricName string) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/DynamoDB"),
		MetricName: aws.String(metricName),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("TableName"), Value: &t.ResourceID}},
		StartTime:  aws.Time(time.Now().Add(-24 * time.Hour)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(60),
		Statistics: []*string{aws.String("SampleCount")},
	}
	result, err := t.cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Printf("DynamoDB.getMetric %v, %s\n", err, *t.cw.Config.Region)
		return -1
	}

	var sum float64
	for _, point := range result.Datapoints {
		sum += *point.SampleCount
	}
	return sum
}
