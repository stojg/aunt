package ec2

import (
	"fmt"
	"strings"
	"time"

	"github.com/ararog/timeago"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stojg/aunt/lib/core"
)

func Headers() []string {
	return []string{"Instance Name", "Credits", "CPU %", "Type", "ResourceID", "Launched", "Region"}
}

func Fetch(region, account, roleARN string) chan core.Resource {
	resources := make(chan core.Resource)
	go func() {
		sess, config := core.NewConfig(region, roleARN)
		svc := ec2.New(sess, config)
		resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String("instance-type"),
					Values: []*string{
						aws.String("t2.nano"),
						aws.String("t2.micro"),
						aws.String("t2.small"),
						aws.String("t2.medium"),
						aws.String("t2.large"),
						aws.String("t2.xlarge"),
						aws.String("t2.2xlarge"),
					},
				},
				{
					Name:   aws.String("instance-state-name"),
					Values: []*string{aws.String("running")},
				},
			},
		})
		if err != nil {
			fmt.Printf("ec2.describeInstances %s %s %v\n", roleARN, region, err)
			return
		}
		for idx := range resp.Reservations {
			for _, ec2Inst := range resp.Reservations[idx].Instances {
				resources <- newInstance(ec2Inst, sess, config)
			}
		}
		close(resources)
	}()
	return resources
}

func newInstance(i *ec2.Instance, sess client.ConfigProvider, config *aws.Config) core.Resource {
	inst := &instance{
		ResourceID:   *i.InstanceId,
		Region:       *config.Region,
		InstanceType: *i.InstanceType,
		LaunchTime:   i.LaunchTime,
		state:        *i.State.Name,
		namespace:    aws.String("AWS/EC2"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: i.InstanceId,
			},
		},
		cw: cloudwatch.New(sess, config),
	}
	if strings.HasPrefix(inst.InstanceType, "t2") {
		inst.burstable = true
	}

	for _, tag := range i.Tags {
		if *tag.Key == "Name" && len(*tag.Value) > 0 {
			inst.Name = *tag.Value
			break
		}
	}
	return inst
}

type instance struct {
	Name             string
	ResourceID       string
	LaunchTime       *time.Time
	Region           string
	InstanceType     string
	CPUUtilization   float64
	CPUCreditBalance float64

	state      string
	burstable  bool
	namespace  *string
	dimensions []*cloudwatch.Dimension
	cw         *cloudwatch.CloudWatch
}

func (i *instance) Row() []string {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *i.LaunchTime)
	return []string{
		i.Name,
		fmt.Sprintf("%.2f", i.CPUCreditBalance),
		fmt.Sprintf("%.2f", i.CPUUtilization),
		i.InstanceType,
		i.ResourceID,
		launchedAgo,
		i.Region,
	}
}

func (i *instance) Display() bool {
	return i.CPUCreditBalance < 10 && i.CPUCreditBalance > 0
}

func (i *instance) SetMetrics() {
	if i.state != "running" {
		return
	}
	if !i.burstable {
		return
	}

	i.CPUUtilization = i.getMetric("CPUUtilization", i.cw)
	i.CPUCreditBalance = i.getMetric("CPUCreditBalance", i.cw)
}

func (i *instance) getMetric(metricName string, cw *cloudwatch.CloudWatch) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  i.namespace,
		MetricName: aws.String(metricName),
		Dimensions: i.dimensions,
		StartTime:  aws.Time(time.Now().Add(-15 * time.Minute)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(3600),
		Statistics: []*string{
			aws.String("Average"),
		},
	}
	result, err := cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Printf("ec2.getMetric %s %v\n", i.Region, err)
		return -1
	}
	if len(result.Datapoints) == 0 {
		return -1
	}
	return *result.Datapoints[0].Average
}
