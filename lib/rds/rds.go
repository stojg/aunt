package rds

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
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stojg/aunt/lib/core"
)

func Get(regions []*string, roles []string) *core.List {
	resourceChans := make([]chan core.Resource, 0)
	for _, roleARN := range roles {
		for _, region := range regions {
			resourceChans = append(resourceChans, fetchDatabases(region, roleARN))
		}
	}
	list := core.NewList()
	for i := range core.Filter(core.Metrics(core.Merge(resourceChans))) {
		list.Add(i)
	}
	return list
}

func fetchDatabases(region *string, roleARN string) chan core.Resource {
	resources := make(chan core.Resource)
	go func() {
		describeRDSes(region, roleARN, resources)
		close(resources)
	}()
	return resources
}

func describeRDSes(region *string, roleARN string, resources chan core.Resource) {
	sess := session.Must(session.NewSession())
	config := &aws.Config{Region: region, Credentials: stscreds.NewCredentials(sess, roleARN)}
	svc := rds.New(sess, config)
	resp, err := svc.DescribeDBInstances(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, db := range resp.DBInstances {
		resources <- newRDS(db, sess, config)
	}
}

func newRDS(db *rds.DBInstance, sess client.ConfigProvider, config *aws.Config) *database {
	r := &database{
		cw:           cloudwatch.New(sess, config),
		ResourceID:   *db.DBInstanceIdentifier,
		Region:       *config.Region,
		InstanceType: *db.DBInstanceClass,
		State:        *db.DBInstanceStatus,
		LaunchTime:   db.InstanceCreateTime,
		name:         strings.Replace(*db.DBInstanceIdentifier, "-", ".", -1) + ".db",
		namespace:    aws.String("AWS/RDS"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("DBInstanceIdentifier"),
				Value: db.DBInstanceIdentifier,
			},
		},
	}
	if strings.Contains(*db.DBInstanceClass, "t2") {
		r.Burstable = true
	}
	return r
}

type database struct {
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

func (i *database) Name() string {
	return i.name
}

func (i *database) Headers() []string {
	return []string{"RDS name", "Credits", "CPU %", "Type", "ResourceID", "Launched", "Region"}
}

func (d *database) SetMetrics() {
	if d.State != "available" {
		return
	}
	if !d.Burstable {
		return
	}
	d.CPUUtilization = d.getMetric("CPUUtilization")
	d.CPUCreditBalance = d.getMetric("CPUCreditBalance")
}

func (i *database) Row() []string {
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

func (d *database) Display() bool {
	if d.State != "available" {
		return false
	}
	if !d.Burstable {
		return false
	}
	return d.CPUCreditBalance < 10
}

func (d *database) getMetric(metricName string) float64 {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  d.namespace,
		MetricName: aws.String(metricName),
		Dimensions: d.dimensions,
		StartTime:  aws.Time(time.Now().Add(-15 * time.Minute)),
		EndTime:    aws.Time(time.Now()),
		Period:     aws.Int64(3600),
		Statistics: []*string{
			aws.String("Average"),
		},
	}
	result, err := d.cw.GetMetricStatistics(input)
	if err != nil {
		fmt.Println(err)
		return -1
	}
	if len(result.Datapoints) == 0 {
		return -1
	}
	return *result.Datapoints[0].Average
}
