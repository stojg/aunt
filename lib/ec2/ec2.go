package ec2

import (
	"fmt"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stojg/aunt/lib/core"
)

// Instance is an app specific representation of a EC2 instance
type Instance struct {
	Name         string
	ResourceID   string `storm:"id"`
	LaunchTime   *time.Time
	Region       string
	Account      string
	InstanceType string
	State        string
	LastUpdated  time.Time
	Metrics      map[string]*float64
}

const (
	metricCredits = "CPUCreditBalance"
	metricsCPU    = "CPUUtilization"
)

const (
	metricsCreditsThreshold float64 = 10
	metricsCPUThreshold     float64 = 90.0
)

var metrics = []string{metricCredits, metricsCPU}

// Update will update the database with Instance data
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
		svc := ec2.New(sess, config)
		cw := cloudwatch.New(sess, config)

		resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("instance-state-name"),
					Values: []*string{aws.String("running")},
				},
			},
		})
		if err != nil {
			fmt.Printf("ec2.describeInstances %s %s %v\n", role, region, err)
			return
		}

		for idx := range resp.Reservations {
			for _, i := range resp.Reservations[idx].Instances {
				instance := &Instance{
					Name:         core.TagValue("Name", i.Tags),
					ResourceID:   *i.InstanceId,
					Region:       *config.Region,
					Account:      account,
					InstanceType: *i.InstanceType,
					LaunchTime:   i.LaunchTime,
					State:        *i.State.Name,
					LastUpdated:  time.Now(),
					Metrics:      make(map[string]*float64),
				}
				dimensions := []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: i.InstanceId}}

				for _, name := range metrics {
					instance.Metrics[name] = metric("AWS/EC2", dimensions, name, cw)
				}
				if err := db.Save(instance); err != nil {
					fmt.Printf("%+v\n", err)
				}
				// check metrics
				credits := instance.Metrics[metricCredits]
				if credits != nil && *credits < metricsCreditsThreshold {
					alert := core.NewAlert(metricCredits, instance.ResourceID)
					alert.Message = fmt.Sprintf("CPU credits (%.1f) is below %.1f for %s", *credits, metricsCreditsThreshold, instance.Name)
					alert.Details["account"] = instance.Account
					alert.Details["region"] = instance.Region
					alert.Details["resource_id"] = instance.ResourceID
					if err := alert.Save(db); err != nil {
						fmt.Printf("%+v\n", err)
					}
				}
				cpu := instance.Metrics[metricsCPU]
				if cpu != nil && *cpu > metricsCPUThreshold {
					alert := core.NewAlert(metricCredits, instance.ResourceID)
					alert.Message = fmt.Sprintf("CPU Utilisation (%.1f) is above %.1f for %s", *cpu, metricsCPUThreshold, instance.Name)
					alert.Details["account"] = instance.Account
					alert.Details["region"] = instance.Region
					alert.Details["resource_id"] = instance.ResourceID
					if err := alert.Save(db); err != nil {
						fmt.Printf("%+v\n", err)
					}
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
