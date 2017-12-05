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
	"github.com/stojg/aunt/lib/stats"
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

// Update will update the database with Instance data
func Update(db *storm.DB, roles map[string]string, regions []string) error {
	fmt.Println(time.Now().Format(time.Kitchen))
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

		statusResp, statusErr := svc.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{})
		if statusErr != nil {
			fmt.Printf("Error during ec2.DescribeInstanceStatus: %v\n", statusErr)
			return
		}

		statuses := make(map[string]string, 0)

		for _, rest := range statusResp.InstanceStatuses {
			systemStatus := *rest.SystemStatus.Status
			instanceStatus := *rest.InstanceStatus.Status
			if systemStatus == "impaired" || instanceStatus == "impaired" {
				statuses[*rest.InstanceId] = fmt.Sprintf("SystemStatus %s, InstanceStatus: %s\n", systemStatus, instanceStatus)
			}
		}

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
			for _, inst := range resp.Reservations[idx].Instances {
				instance := &Instance{
					Name:         core.TagValue("Name", inst.Tags),
					ResourceID:   *inst.InstanceId,
					Region:       *config.Region,
					Account:      account,
					InstanceType: *inst.InstanceType,
					LaunchTime:   inst.LaunchTime,
					State:        *inst.State.Name,
					LastUpdated:  time.Now(),
					Metrics:      make(map[string]*float64),
				}

				dimensions := []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: inst.InstanceId}}
				dataPoints := cpuCreditsMetrics("AWS/EC2", dimensions, cw)
				if len(dataPoints) < 3 {
					continue
				}

				linear := stats.NewLinear(stats.Convert(dataPoints))
				zeroAt := linear.AtY(0)
				now := time.Now()
				creditUsagePerHour := linear.Slope() * 3600
				launchTime := instance.LaunchTime.Local().Format("2006-01-02 15:04")
				// priority 1
				if zeroAt.After(now) && zeroAt.Before(now.Add(6*time.Hour)) {
					fmt.Printf("P1: %shrs %s (%s, %s) | %+0.2fcr/hr | current: %0.2f | launched: %s\n", zeroAt.Sub(now), instance.Name, instance.ResourceID, instance.Region, creditUsagePerHour, linear.LastY(), launchTime)
				} else if linear.LastY() < 10 && linear.Slope() < 10 {
					fmt.Printf("P2: %0.2fcrds %s (%s, %s) | %+0.2fcr/hr | launched: %s\n", linear.LastY(), instance.Name, instance.ResourceID, instance.Region, creditUsagePerHour, launchTime)
				}

				if val := statuses[instance.ResourceID]; val != "" {
					fmt.Printf("P2: %s (%s, %s) %s | launched: %s\n", instance.Name, instance.ResourceID, instance.Region, val, launchTime)
				}

				//for _, name := range metrics {
				//	instance.Metrics[name] = cpuCreditsMetrics("AWS/EC2", dimensions, name, cw)
				//}
				//if err := db.Save(instance); err != nil {
				//	fmt.Printf("%+v\n", err)
				//}
				//// check metrics
				//credits := instance.Metrics[metricCredits]
				//if credits != nil && *credits < metricsCreditsThreshold {
				//	alert := core.NewAlert(metricCredits, instance.ResourceID)
				//	alert.Message = fmt.Sprintf("CPU credits (%.1f) is below %.1f for %s", *credits, metricsCreditsThreshold, instance.Name)
				//	alert.Details["account"] = instance.Account
				//	alert.Details["region"] = instance.Region
				//	alert.Details["resource_id"] = instance.ResourceID
				//	if err := alert.Save(db); err != nil {
				//		fmt.Printf("%+v\n", err)
				//	}
				//}
				//cpu := instance.Metrics[metricsCPU]
				//if cpu != nil && *cpu > metricsCPUThreshold {
				//	alert := core.NewAlert(metricCredits, instance.ResourceID)
				//	alert.Message = fmt.Sprintf("CPU Utilisation (%.1f) is above %.1f for %s", *cpu, metricsCPUThreshold, instance.Name)
				//	alert.Details["account"] = instance.Account
				//	alert.Details["region"] = instance.Region
				//	alert.Details["resource_id"] = instance.ResourceID
				//	if err := alert.Save(db); err != nil {
				//		fmt.Printf("%+v\n", err)
				//	}
				//}
			}
		}
	}
}

func cpuCreditsMetrics(namespace string, dimensions []*cloudwatch.Dimension, cw *cloudwatch.CloudWatch) []*cloudwatch.Datapoint {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String("CPUCreditBalance"),
		Dimensions: dimensions,
		StartTime:  aws.Time(time.Now().Add(-6 * time.Hour)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(300),
		Statistics: []*string{aws.String("Average")},
	}
	result, err := cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Printf("ec2.getMetric %v\n", err)
		return nil
	}
	return result.Datapoints
}
