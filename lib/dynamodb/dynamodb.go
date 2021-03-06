package dynamodb

import (
	"fmt"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stojg/aunt/lib/core"
)

// Table is a app specific representation of a dynamodb table
type Table struct {
	Name          string
	ResourceID    string `storm:"id"`
	LaunchTime    *time.Time
	Region        string
	Account       string
	LastUpdated   time.Time
	Metrics       map[string]*float64
	Entries       int64
	WriteCapacity int64
	ReadCapacity  int64
}

const (
	readThrottleEvents  = "ReadThrottleEvents"
	writeThrottleEvents = "WriteThrottleEvents"
)

const (
	readThrottleEventsThreshold  float64 = 10
	writeThrottleEventsThreshold float64 = 10
)

var metrics = []string{readThrottleEvents, writeThrottleEvents}

// Update will update the database with Table data
func Update(db *storm.DB, roles map[string]string, regions []string) error {
	var wg sync.WaitGroup
	wg.Add(len(roles))

	for account, role := range roles {
		// update all accounts in parallel to speed this up
		go func(account, role string) {
			updateForRole(db, account, role, regions)
			wg.Done()
		}(account, role)
	}
	wg.Wait()
	return nil
}

func updateForRole(db *storm.DB, account, role string, regions []string) {
	for _, region := range regions {
		sess, config := core.NewCredentials(region, role)
		svc := dynamodb.New(sess, config)
		cw := cloudwatch.New(sess, config)

		resp, err := svc.ListTables(nil)
		if err != nil {
			fmt.Printf("dynamodb.ListTables %s %s %v\n", role, region, err)
			return
		}

		for _, tableName := range resp.TableNames {
			data, err := svc.DescribeTable(&dynamodb.DescribeTableInput{TableName: tableName})
			if err != nil {
				fmt.Printf("dynamodb.FetchTables.DescribeTable - %s - %s %v\n", role, region, err)
				continue
			}

			table := &Table{
				Name:          *tableName,
				ResourceID:    *tableName,
				LaunchTime:    data.Table.CreationDateTime,
				Entries:       *data.Table.ItemCount,
				Region:        *config.Region,
				Account:       account,
				LastUpdated:   time.Now(),
				Metrics:       make(map[string]*float64),
				WriteCapacity: *data.Table.ProvisionedThroughput.WriteCapacityUnits,
				ReadCapacity:  *data.Table.ProvisionedThroughput.ReadCapacityUnits,
			}

			dimensions := []*cloudwatch.Dimension{{Name: aws.String("TableName"), Value: tableName}}

			for _, name := range metrics {
				table.Metrics[name] = metric("AWS/DynamoDB", dimensions, name, cw)
			}
			if err := db.Save(table); err != nil {
				fmt.Printf("%+v\n", err)
			}

			// check metrics
			throttledReads := table.Metrics[readThrottleEvents]
			if throttledReads != nil && *throttledReads > readThrottleEventsThreshold {
				alert := core.NewAlert(readThrottleEvents, table.ResourceID)
				alert.Message = fmt.Sprintf("Throttled reads (%.1f) is above %.1f for %s", *throttledReads, readThrottleEventsThreshold, table.ResourceID)
				alert.Details["account"] = table.Account
				alert.Details["region"] = table.Region
				alert.Details["resource_id"] = table.ResourceID
				if err := alert.Save(db); err != nil {
					fmt.Printf("%+v\n", err)
				}
			}
			throttledWrites := table.Metrics[writeThrottleEvents]
			if throttledWrites != nil && *throttledWrites > writeThrottleEventsThreshold {
				alert := core.NewAlert(writeThrottleEvents, table.ResourceID)
				alert.Message = fmt.Sprintf("Throttled writes (%.1f) is above %.1f for %s", *throttledWrites, writeThrottleEventsThreshold, table.ResourceID)
				alert.Details["account"] = table.Account
				alert.Details["region"] = table.Region
				alert.Details["resource_id"] = table.ResourceID
				if err := alert.Save(db); err != nil {
					fmt.Printf("%+v\n", err)
				}
			}
		}
	}
}

func metric(namespace string, dimensions []*cloudwatch.Dimension, metricName string, cw *cloudwatch.CloudWatch) *float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: dimensions,
		StartTime:  aws.Time(time.Now().Add(-15 * time.Minute)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(3600),
		Statistics: []*string{aws.String("Average")},
	}
	result, err := cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Printf("ec2.getMetric %v\n", err)
		return nil
	}
	if len(result.Datapoints) == 0 {
		return nil
	}
	return result.Datapoints[0].Average
}
