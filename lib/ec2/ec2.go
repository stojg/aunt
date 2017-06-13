package ec2

import (
	"fmt"
	"strings"
	"time"

	"github.com/ararog/timeago"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stojg/aunt/lib/core"
)

func Get(regions []*string, roles []string) *core.List {
	resourceChans := make([]chan core.Resource, 0)
	for _, roleARN := range roles {
		for _, region := range regions {
			resourceChans = append(resourceChans, fetchInstances(region, roleARN))
		}
	}
	list := core.NewList()
	for i := range core.Filter(core.Metrics(core.Merge(resourceChans))) {
		list.Add(i)
	}
	return list
}

func fetchInstances(region *string, roleARN string) chan core.Resource {
	resources := make(chan core.Resource)
	go func() {
		describeInstances(region, roleARN, resources)
		close(resources)
	}()
	return resources
}

func describeInstances(region *string, roleARN string, resources chan core.Resource) {
	sess := session.Must(session.NewSession())
	config := &aws.Config{Region: region, Credentials: stscreds.NewCredentials(sess, roleARN)}
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
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	for idx := range resp.Reservations {
		for _, ec2Inst := range resp.Reservations[idx].Instances {
			resources <- newInstance(ec2Inst, sess, config)
		}
	}
}

func newInstance(i *ec2.Instance, sess client.ConfigProvider, config *aws.Config) core.Resource {
	cw := cloudwatch.New(sess, config)

	r := &instance{
		ResourceID:   *i.InstanceId,
		Region:       *config.Region,
		cw:           cw,
		InstanceType: *i.InstanceType,
		State:        *i.State.Name,
		LaunchTime:   i.LaunchTime,
		namespace:    aws.String("AWS/EC2"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: i.InstanceId,
			},
		},
	}
	if strings.HasPrefix(r.InstanceType, "t2") {
		r.Burstable = true
	}

	for _, tag := range i.Tags {
		if *tag.Key == "Name" && len(*tag.Value) > 0 {
			r.name = *tag.Value
			break
		}
	}
	return r
}

type instance struct {
	ResourceID       string
	LaunchTime       *time.Time
	name             string
	InstanceType     string
	Region           string
	State            string
	Burstable        bool
	CPUUtilization   float64
	CPUCreditBalance float64
	namespace        *string
	dimensions       []*cloudwatch.Dimension
	cw               *cloudwatch.CloudWatch
}

func (i *instance) Name() string {
	return i.name
}

func (i *instance) Headers() []string {
	return []string{"Instance Name", "Credits", "CPU %", "Type", "ResourceID", "Launched", "Region"}
}

func (i *instance) Row() []string {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *i.LaunchTime)
	return []string{
		i.name,
		fmt.Sprintf("%.2f", i.CPUCreditBalance),
		fmt.Sprintf("%.2f", i.CPUUtilization),
		i.InstanceType,
		i.ResourceID,
		launchedAgo,
		i.Region,
	}
}

func (i *instance) Display() bool {
	return i.CPUCreditBalance < 10
}

func (i *instance) SetMetrics() {
	if i.State != "running" {
		return
	}
	if !i.Burstable {
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
		fmt.Println(err)
		return -1
	}
	if len(result.Datapoints) == 0 {
		return -1
	}
	return *result.Datapoints[0].Average
}
