package ebs

import (
	"fmt"

	"time"

	"github.com/ararog/timeago"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stojg/aunt/lib/core"
)

// Headers returns a list of headers for use in a table header
func Headers() []string {
	return []string{"Instance Name", "Min BurstBalance (24hr)", "Size", "IOPS", "ResourceID", "Launched", "Account", "AZ", "Attached To"}
}

// Fetch returns a channel where it will asynchronously will send Resources
func Fetch(region, account, roleARN string) chan core.Resource {
	out := make(chan core.Resource, 16)
	go func() {
		sess, config := core.NewConfig(region, roleARN)
		svc := ec2.New(sess, config)

		resp, err := svc.DescribeVolumes(nil)
		if err != nil {
			fmt.Printf("dynamodb.Fetch.ListTables - %s - %s %v\n", region, account, err)
			return
		}
		for _, volume := range resp.Volumes {
			out <- newVolume(volume, sess, config, account)
		}
		close(out)
	}()
	return out
}

func newVolume(data *ec2.Volume, sess client.ConfigProvider, config *aws.Config, account string) core.Resource {

	v := &volume{
		cw:               cloudwatch.New(sess, config),
		Name:             *data.VolumeId,
		ResourceID:       *data.VolumeId,
		LaunchTime:       data.CreateTime,
		Account:          account,
		AvailabilityZone: *data.AvailabilityZone,
		size:             *data.Size,
		namespace:        aws.String("AWS/EBS"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("VolumeId"),
				Value: data.VolumeId,
			},
		},
	}

	if len(data.Attachments) > 1 {
		fmt.Println("multiple attachments")
	}
	// in reality, there is only one attachment
	for _, attachment := range data.Attachments {
		if *attachment.State == "attached" {
			v.attached = true
		}
		v.InstanceID = *attachment.InstanceId
	}

	if data.Iops != nil {
		v.iops = *data.Iops
	}

	for _, tag := range data.Tags {
		if *tag.Key == "Name" && len(*tag.Value) > 0 {
			v.Name = *tag.Value
			break
		}
	}

	return v
}

type volume struct {
	Name             string
	ResourceID       string
	LaunchTime       *time.Time
	AvailabilityZone string
	BurstBalance     float64
	InstanceID       string
	Account          string

	size       int64
	iops       int64
	attached   bool
	namespace  *string
	dimensions []*cloudwatch.Dimension
	cw         *cloudwatch.CloudWatch
}

func (v *volume) Row() []string {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *v.LaunchTime)
	return []string{
		v.Name,
		fmt.Sprintf("%0.1f", v.BurstBalance),
		fmt.Sprintf("%0d", v.size),
		fmt.Sprintf("%d", v.iops),
		v.ResourceID,
		launchedAgo,
		v.Account,
		v.AvailabilityZone,
		v.InstanceID,
	}
}

func (v *volume) Display() bool {
	return v.BurstBalance < 90 && v.BurstBalance > 0
}

func (v *volume) SetMetrics() {
	v.BurstBalance = v.getMetric("BurstBalance", v.cw)
}

func (v *volume) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  v.namespace,
		MetricName: aws.String(metricName),
		Dimensions: v.dimensions,
		StartTime:  aws.Time(time.Now().Add(-24 * time.Hour)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(60),
		Statistics: []*string{
			aws.String("Average"),
		},
	}
	result, err := cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Printf("ebs.getMetric %s %s %v\n", v.Account, v.AvailabilityZone, err)
		return -1
	}
	if len(result.Datapoints) == 0 {
		return -1
	}
	var lowestValue float64 = 100
	for _, point := range result.Datapoints {
		if *point.Average < lowestValue {
			lowestValue = *point.Average
		}
	}
	return lowestValue
}
