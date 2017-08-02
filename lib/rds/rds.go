package rds

import (
	"fmt"
	"time"

	"sync"

	"strings"

	"sort"

	"github.com/ararog/timeago"
	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stojg/aunt/lib/core"
)

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
	MetricCredits = "CPUCreditBalance"
	MetricsCPU    = "CPUUtilization"
)

const (
	MetricsCreditsThreshold float64 = 30
	MetricsCPUThreshold     float64 = 70.0
)

var metrics = []string{MetricCredits, MetricsCPU}

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

func Purge(db *storm.DB, olderThan time.Duration) error {
	var resources []DBInstance

	startTime := time.Time{}
	endTime := time.Now().Add(-1 * olderThan)

	if err := db.Range("LastUpdated", startTime, endTime, &resources); err != nil && err != storm.ErrNotFound {
		return err
	}

	for _, i := range resources {
		fmt.Printf("purging: %s %s\n", i.Name, i.LastUpdated)
		// @todo, close all alerts related to this instance
		if err := db.DeleteStruct(&i); err != nil {
			fmt.Printf("purge: %v %s\n", err, i.Name)
		}
	}
	return nil
}

func TableData(db *storm.DB) ([]string, [][]string, error) {
	var headers []string
	var rows [][]string

	var instances []DBInstance
	if err := db.All(&instances); err != nil {
		return headers, rows, err
	}
	if len(instances) < 1 {
		return headers, rows, nil
	}

	headers = asData(instances[0]).Keys()
	for _, i := range instances {
		rows = append(rows, asData(i).Values())
	}
	sort.Sort(core.RowSorter(rows))
	return headers, rows, nil
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
			credits := instance.Metrics[MetricCredits]
			if credits != nil && *credits < MetricsCreditsThreshold {
				alert := core.NewAlert(MetricCredits, instance.ResourceID)
				alert.Message = fmt.Sprintf("CPU credits (%.1f) is below %.1f for %s", *credits, MetricsCreditsThreshold, instance.Name)
				alert.Details["account"] = instance.Account
				alert.Details["region"] = instance.Region
				alert.Details["resource_id"] = instance.ResourceID
				if err := alert.Save(db); err != nil {
					fmt.Printf("%+v\n", err)
				}
			}
			cpu := instance.Metrics[MetricsCPU]
			if cpu != nil && *cpu > MetricsCPUThreshold {
				alert := core.NewAlert(MetricCredits, instance.ResourceID)
				alert.Message = fmt.Sprintf("CPU Utilisation (%.1f) is above %.1f for %s", *cpu, MetricsCPUThreshold, instance.Name)
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

func asData(i DBInstance) core.KeyValue {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *i.LaunchTime)
	var d core.KeyValue
	d.Add("DBInstance Name", i.Name)
	for _, name := range metrics {
		val := i.Metrics[name]
		if val == nil {
			d.Add(name, "")
		} else {
			d.Add(name, fmt.Sprintf("%.2f", *val))
		}
	}
	d.Add("Type", i.InstanceType)
	d.Add("ResourceID", i.ResourceID)
	d.Add("Launched", launchedAgo)
	d.Add("Region", i.Region)
	d.Add("Account", i.Account)
	return d
}
