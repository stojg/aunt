package asg

import (
	"fmt"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stojg/aunt/lib/core"
	//"strings"
)

type AutoScalingGroup struct {
	Name        string
	ResourceID  string `storm:"id"`
	Region      string
	Account     string
	LastUpdated time.Time
	Metrics     map[string]*float64
}

const (
	MetricNumEvents = "NumScalingEvents"
)

const (
	MetricNumEventsThreshold float64 = 10
)

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
		svc := autoscaling.New(sess, config)

		resp, err := svc.DescribeAutoScalingGroups(nil)
		if err != nil {
			fmt.Printf("asg.DescribeAutoScalingGroups %s %s %v\n", role, region, err)
			return
		}

		for _, data := range resp.AutoScalingGroups {
			resp, err := svc.DescribeScalingActivities(&autoscaling.DescribeScalingActivitiesInput{
				AutoScalingGroupName: data.AutoScalingGroupName,
				MaxRecords:           aws.Int64(100),
			})

			if err != nil {
				fmt.Printf("asg.DescribeScalingActivities %s %s %v\n", role, region, err)
				return
			}

			asg := &AutoScalingGroup{
				Name:        *data.AutoScalingGroupName,
				ResourceID:  *data.AutoScalingGroupName,
				Region:      region,
				Account:     account,
				LastUpdated: time.Now(),
				Metrics:     make(map[string]*float64),
			}

			asg.Metrics[MetricNumEvents] = aws.Float64(0)

			since := time.Now().Add(-3 * time.Hour)
			description := ""
			for _, act := range resp.Activities {
				if act.StartTime.After(since) {
					*asg.Metrics[MetricNumEvents]++
					description = fmt.Sprintf("%s %s - %s\n", description, act.StartTime.Local(), *act.Description)
				}
			}

			if err := db.Save(asg); err != nil {
				fmt.Printf("%+v\n", err)
			}

			if *asg.Metrics[MetricNumEvents] >= MetricNumEventsThreshold {
				alert := core.NewAlert(MetricNumEvents, asg.ResourceID)
				alert.Message = fmt.Sprintf("ASG %s has %0.f scaling events in the last 3 hours, threshold %.0f", asg.Name, *asg.Metrics[MetricNumEvents], MetricNumEventsThreshold)
				alert.Details["account"] = asg.Account
				alert.Details["region"] = asg.Region
				alert.Details["resource_id"] = asg.ResourceID
				alert.Details["num_scaling_events"] = fmt.Sprintf("%.0f", *asg.Metrics[MetricNumEvents])
				alert.Description = description
				if err := alert.Save(db); err != nil {
					fmt.Printf("%+v\n", err)
				}
			}
		}
	}
}
