package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"time"
)

func update(regions []*string) chan *Instance {

	instances := make([]chan *Instance, 0)
	for _, region := range regions {
		instances = append(instances, getResources(region))
	}

	merged := merge(instances)
	metrics := fetchMetrics(merged)

	return metrics

}

func getResources(region *string) chan *Instance {
	resources := make(chan *Instance)
	go func() {
		describeInstances(region, resources)
		describeRDSes(region, resources)
		close(resources)
	}()
	return resources
}

func fetchMetrics(instances chan *Instance) chan *Instance {

	out := make(chan *Instance)

	go func() {
		for instance := range instances {
			if instance.State != "running" && instance.State != "available" {
				continue
			}

			if !instance.Burstable {
				continue
			}

			cw := cloudwatch.New(session.New(), &aws.Config{Region: aws.String(instance.Region)})

			point, err := getMetric(instance, "CPUUtilization", cw)
			if err != nil {
				fmt.Printf("%s\n", err)
			}
			instance.CPUUtilization = point

			point, err = getMetric(instance, "CPUCreditBalance", cw)
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

func describeInstances(region *string, resources chan *Instance) {
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

func describeRDSes(region *string, resources chan *Instance) {
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

func getMetric(resource *Instance, metricName string, cw *cloudwatch.CloudWatch) (float64, error) {
	endTime := time.Now()
	startTime := endTime.Add(-15 * time.Minute)
	period := int64(3600)
	metrics, err := cw.GetMetricStatistics(&cloudwatch.GetMetricStatisticsInput{
		Namespace:  resource.Namespace,
		MetricName: aws.String(metricName),
		Dimensions: resource.Dimensions,
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
	return -1, fmt.Errorf("No datapoints for %s and metric %s", resource.ResourceID, metricName)
}
