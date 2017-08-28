package rds

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stojg/aunt/lib/core"
)

// DBInstance contains app specific information about an RDS
type DBInstance struct {
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

// different threshold depednding on db instance type
const (
	metricsCreditsThreshold float64 = 15
	metricsCPUThreshold     float64 = 70.0
)

var metrics = []string{metricCredits, metricsCPU}

// Update will update the database with DBInstance data
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
		svc := rds.New(sess, config)
		cw := cloudwatch.New(sess, config)

		resp, err := svc.DescribeDBInstances(nil)
		if err != nil {
			fmt.Printf("rds.DescribeDBInstances %s %s %v\n", role, region, err)
			return
		}

		for _, i := range resp.DBInstances {
			instance := &DBInstance{
				Name:         strings.Replace(*i.DBInstanceIdentifier, "-", ".", -1) + ".db",
				ResourceID:   *i.DBInstanceIdentifier,
				Region:       *config.Region,
				Account:      account,
				InstanceType: *i.DBInstanceClass,
				LaunchTime:   i.InstanceCreateTime,
				State:        *i.DBInstanceStatus,
				LastUpdated:  time.Now(),
				Metrics:      make(map[string]*float64),
			}

			dimensions := []*cloudwatch.Dimension{{Name: aws.String("DBInstanceIdentifier"), Value: i.DBInstanceIdentifier}}

			for _, name := range metrics {
				instance.Metrics[name] = metric("AWS/RDS", dimensions, name, cw)
			}
			if err := db.Save(instance); err != nil {
				fmt.Printf("%+v\n", err)
			}

			//check metrics
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
		fmt.Printf("ec2.metric %v\n", err)
		return nil
	}
	if len(result.Datapoints) == 0 {
		return nil
	}
	return result.Datapoints[0].Average
}
