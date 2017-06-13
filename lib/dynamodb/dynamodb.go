package dynamodb

import (
	"fmt"
	"time"

	"github.com/ararog/timeago"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stojg/aunt/lib/core"
)

func Get(regions []*string, roles []string) *core.List {
	resourceChans := make([]chan core.Resource, 0)
	for _, roleARN := range roles {
		for _, region := range regions {
			resourceChans = append(resourceChans, fetchTables(region, roleARN))
		}
	}

	list := core.NewList()
	for i := range core.Filter(core.Metrics(core.Merge(resourceChans))) {
		list.Add(i)
	}
	return list
}

func fetchTables(region *string, roleARN string) chan core.Resource {
	out := make(chan core.Resource, 16)
	go func() {
		sess := session.Must(session.NewSession(&aws.Config{Region: region}))
		config := &aws.Config{Credentials: stscreds.NewCredentials(sess, roleARN), Region: region}
		svc := dynamodb.New(sess, config)
		resp, err := svc.ListTables(nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, t := range resp.TableNames {
			tableDesc, err := svc.DescribeTable(&dynamodb.DescribeTableInput{
				TableName: t,
			})
			if err != nil {
				fmt.Println(err)
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
		ResourceID:    *t.TableName,
		LaunchTime:    t.CreationDateTime,
		name:          *t.TableName,
		Region:        *config.Region,
		Items:         *t.ItemCount,
		WriteCapacity: *t.ProvisionedThroughput.WriteCapacityUnits,
		ReadCapacity:  *t.ProvisionedThroughput.ReadCapacityUnits,
		cw:            cw,
	}
}

type table struct {
	ResourceID          string
	LaunchTime          *time.Time
	Items               int64
	name                string
	Region              string
	RoleARN             string
	WriteThrottleEvents float64
	WriteCapacity       int64
	ReadThrottleEvents  float64
	ReadCapacity        int64
	cw                  *cloudwatch.CloudWatch
}

func (t table) Headers() []string {
	return []string{"DynamoDB name", "Read Capacity", "Throttled read events (24hr)", "Write Capacity", "Throttled write events (24hr)", "Entries", "Launched", "Region"}
}

func (t *table) Name() string {
	return t.name
}

func (t *table) Row() []string {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *t.LaunchTime)
	return []string{
		t.name,
		fmt.Sprintf("%d", t.ReadCapacity),
		fmt.Sprintf("%.0f", t.ReadThrottleEvents),
		fmt.Sprintf("%d", t.WriteCapacity),
		fmt.Sprintf("%.0f", t.WriteThrottleEvents),
		fmt.Sprintf("%d", t.Items),
		launchedAgo,
		t.Region,
	}
}

func (t *table) Display() bool {
	return t.ReadThrottleEvents > 0 || t.WriteThrottleEvents > 0 || t.Items > 3600
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
		fmt.Println(err)
		return -1
	}

	var sum float64
	for _, point := range result.Datapoints {
		sum += *point.SampleCount
	}
	return sum
}
