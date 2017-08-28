package asg

import (
	"fmt"
	"sync"
	"time"

	"strings"

	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stojg/aunt/lib/core"
)

// AutoScalingGroup contains app specific data for auto scaling groups
type AutoScalingGroup struct {
	Name        string
	ResourceID  string `storm:"id"`
	Region      string
	Account     string
	LastUpdated time.Time
	Metrics     map[string]*float64
}

const (
	metricNumEvents = "NumScalingEvents"
)

const (
	metricNumEventsThreshold float64 = 6
)

// Update will update the database with AutoScalingGroup data
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

			asg.Metrics[metricNumEvents] = aws.Float64(0)

			since := time.Now().Add(-2 * time.Hour)
			description := ""
			for _, a := range resp.Activities {
				if a.StartTime.After(since) {
					if a.Cause != nil {
						if !strings.Contains(*a.Cause, "user request") && !strings.Contains(*a.Cause, "a scheduled action update") {
							*asg.Metrics[metricNumEvents]++
							description = fmt.Sprintf("%s %s - %s\n", description, a.StartTime.Local(), *a.Description)
							description = fmt.Sprintf("%s %s - %s\n", description, a.StartTime.Local(), *a.Cause)
						}
					}
				}
			}

			if err := db.Save(asg); err != nil {
				fmt.Printf("%+v\n", err)
			}

			if *asg.Metrics[metricNumEvents] >= metricNumEventsThreshold {
				alert := core.NewAlert(metricNumEvents, asg.ResourceID)
				alert.Message = fmt.Sprintf("ASG %s has %0.f unexpected events in the last 2 hours, threshold %.0f", asg.Name, *asg.Metrics[metricNumEvents], metricNumEventsThreshold)
				alert.Details["account"] = asg.Account
				alert.Details["region"] = asg.Region
				alert.Details["resource_id"] = asg.ResourceID
				alert.Details["num_scaling_events"] = fmt.Sprintf("%.0f", *asg.Metrics[metricNumEvents])
				alert.Description = description
				if err := alert.Save(db); err != nil {
					fmt.Printf("%+v\n", err)
				}
			}
		}
	}
}
