package rds

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"strings"
	"time"
)

func Get(regions []*string) *List {
	dynamoDBs := make([]chan *database, 0)
	for _, region := range regions {
		dynamoDBs = append(dynamoDBs, fetchDatabases(region))
	}
	merged := merge(dynamoDBs)
	metrics := metrics(merged)
	filtered := filter(metrics, 20)

	// drain channel
	list := NewList()
	for i := range filtered {
		list.items = append(list.items, i)
	}
	return list
}

func newRDS(db *rds.DBInstance, region *string) *database {
	r := &database{
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

type database struct {
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

func (d *database) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  d.namespace,
		MetricName: aws.String(metricName),
		Dimensions: d.dimensions,
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
