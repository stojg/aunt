package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"time"
)

func fetchDynamoDBs(regions []*string) chan *Dynamodb {
	dynamoDBs := make([]chan *Dynamodb, 0)
	for _, region := range regions {
		dynamoDBs = append(dynamoDBs, getDynamoDBs(region))
	}
	merged := mergeDynamoDBs(dynamoDBs)
	metrics := fetchDynamoDBMetrics(merged)
	return metrics
}

func fetchResources(regions []*string) chan *Resource {
	r := make([]chan *Resource, 0)
	for _, region := range regions {
		r = append(r, getResources(region))
	}
	merged := mergeResources(r)
	metrics := fetchResourceMetrics(merged)
	return metrics
}

func getDynamoDBs(region *string) chan *Dynamodb {
	resources := make(chan *Dynamodb)
	go func() {
		describeDynamoDBs(region, resources)
		close(resources)
	}()
	return resources
}

func getResources(region *string) chan *Resource {
	resources := make(chan *Resource)
	go func() {
		describeInstances(region, resources)
		describeRDSes(region, resources)
		close(resources)
	}()
	return resources
}

func fetchResourceMetrics(instances chan *Resource) chan *Resource {

	out := make(chan *Resource)

	go func() {
		for instance := range instances {
			if instance.State != "running" && instance.State != "available" {
				continue
			}
			if !instance.Burstable {
				continue
			}
			cw := cloudwatch.New(session.New(), &aws.Config{Region: aws.String(instance.Region)})

			point, err := getResourceMetric(instance, "CPUUtilization", cw)
			if err != nil {
				fmt.Printf("%s\n", err)
			}
			instance.CPUUtilization = point

			point, err = getResourceMetric(instance, "CPUCreditBalance", cw)
			if err != nil {
				fmt.Printf("%s\n", err)
			}
			instance.CPUCreditBalance = point

			out <- instance
		}
		close(out)
	}()

	return out
}

func describeDynamoDBs(region *string, tables chan *Dynamodb) {
	sess := session.New()
	svc := dynamodb.New(sess, &aws.Config{Region: region})
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
			return
		}
		tables <- NewDynamoDB(tableDesc.Table, region)
	}
}

func describeInstances(region *string, resources chan *Resource) {
	sess := session.New()
	svc := ec2.New(sess, &aws.Config{Region: region})
	resp, err := svc.DescribeInstances(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for idx := range resp.Reservations {
		for _, ec2Inst := range resp.Reservations[idx].Instances {
			resources <- NewInstance(ec2Inst, region)
		}
	}
}

func describeRDSes(region *string, resources chan *Resource) {
	sess := session.New()
	svc := rds.New(sess, &aws.Config{Region: region})
	resp, err := svc.DescribeDBInstances(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, db := range resp.DBInstances {
		resources <- NewRDS(db, region)
	}
}

type Metricable interface {
	ID() string
	Namespace() *string
	Dimensions() []*cloudwatch.Dimension
}

func fetchDynamoDBMetrics(tables chan *Dynamodb) chan *Dynamodb {
	out := make(chan *Dynamodb)
	go func() {
		for table := range tables {
			cw := cloudwatch.New(session.New(), &aws.Config{Region: aws.String(table.Region)})

			point, err := getDynamodbMetric(table, "WriteThrottleEvents", cw)
			if err != nil {
				fmt.Printf("%s\n", err)
			}
			table.WriteThrottleEvents = point

			point, err = getDynamodbMetric(table, "ReadThrottleEvents", cw)
			if err != nil {
				fmt.Printf("%s\n", err)
			}
			table.ReadThrottleEvents = point
			out <- table
		}
		close(out)
	}()
	return out
}

func getDynamodbMetric(resource Metricable, metricName string, cw *cloudwatch.CloudWatch) (float64, error) {
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)
	period := int64(3600)
	metrics, err := cw.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
		Namespace:  resource.Namespace(),
		MetricName: aws.String(metricName),
		Dimensions: resource.Dimensions(),
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     &period,
		Statistics: []*string{
			aws.String("SampleCount"),
		},
	})
	if err != nil {
		return 0, err
	}
	if len(metrics.Datapoints) > 0 {
		return *metrics.Datapoints[0].SampleCount, nil
	}
	return 0, nil
}

func getResourceMetric(resource Metricable, metricName string, cw *cloudwatch.CloudWatch) (float64, error) {
	endTime := time.Now()
	startTime := endTime.Add(-15 * time.Minute)
	period := int64(3600)
	metrics, err := cw.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
		Namespace:  resource.Namespace(),
		MetricName: aws.String(metricName),
		Dimensions: resource.Dimensions(),
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     &period,
		Statistics: []*string{
			aws.String("Average"),
		},
	})

	if err != nil {
		return 0, err
	}

	if len(metrics.Datapoints) > 0 {
		return *metrics.Datapoints[0].Average, nil
	}
	return -1, nil
}
