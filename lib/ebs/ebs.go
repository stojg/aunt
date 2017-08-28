package ebs

import (
	"fmt"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stojg/aunt/lib/core"
	auntec2 "github.com/stojg/aunt/lib/ec2"
)

// Volume is an app specific representation of a EBS volume
type Volume struct {
	Name        string
	ResourceID  string `storm:"id"`
	InstanceID  string
	LaunchTime  *time.Time
	Region      string
	Account     string
	Size        int64
	IOPS        *int64
	Attached    bool
	State       string
	LastUpdated time.Time
	Metrics     map[string]*float64
}

const (
	metricBurstBalance = "BurstBalance"
)

const (
	metricBurstBalanceThreshold float64 = 20
)

var metrics = []string{metricBurstBalance}

// Update will update the database with Volume data
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

		resp, err := svc.DescribeVolumes(nil)
		if err != nil {
			fmt.Printf("ec2.DescribeVolumes %s %s %v\n", role, region, err)
			return
		}

		for _, data := range resp.Volumes {

			volume := &Volume{
				Name:        core.TagValue("Name", data.Tags),
				ResourceID:  *data.VolumeId,
				LaunchTime:  data.CreateTime,
				Region:      *config.Region,
				Account:     account,
				Size:        *data.Size,
				LastUpdated: time.Now(),
				Metrics:     make(map[string]*float64),
			}

			// practically a volume can only be attached to one instance at the time, but it's still an slice.
			for _, attachment := range data.Attachments {
				if *attachment.State == "attached" {
					volume.Attached = true
					volume.InstanceID = *attachment.InstanceId
				}
			}

			// some volumes aren't tagged with a name, try grab it from the attached instance
			if volume.Name == "" && volume.Attached {
				var inst auntec2.Instance
				err := db.One("ResourceID", volume.InstanceID, &inst)
				if err == nil {
					volume.Name = fmt.Sprintf("%s.assets", inst.Name)
				} else if err != storm.ErrNotFound {
					fmt.Printf("Error during instance name lookup: %+v\n", err)
				}
			}

			if data.Iops != nil {
				volume.IOPS = data.Iops
			}

			dimensions := []*cloudwatch.Dimension{{Name: aws.String("VolumeId"), Value: data.VolumeId}}

			for _, name := range metrics {
				volume.Metrics[name] = metric("AWS/EBS", dimensions, name, cw)
			}
			if err := db.Save(volume); err != nil {
				fmt.Printf("%+v\n", err)
			}

			// check metrics
			balance := volume.Metrics[metricBurstBalance]
			if balance != nil && *balance < metricBurstBalanceThreshold {
				alert := core.NewAlert(metricBurstBalance, volume.ResourceID)
				alert.Message = fmt.Sprintf("Burst balance (%.1f%%) is below %.1f%% for volume %s", *balance, metricBurstBalanceThreshold, volume.Name)
				alert.Details["account"] = volume.Account
				alert.Details["region"] = volume.Region
				alert.Details["resource_id"] = volume.ResourceID
				alert.Details["iops"] = fmt.Sprintf("%d", *volume.IOPS)
				alert.Details["size"] = fmt.Sprintf("%d", volume.Size)
				alert.Details["attached_to"] = volume.InstanceID
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
