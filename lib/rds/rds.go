package rds

import (
	"fmt"
	"strings"
	"time"

	"github.com/ararog/timeago"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stojg/aunt/lib/core"
)

// Headers returns a list of headers for use in a table header
func Headers() []string {
	return []string{"RDS Name", "Credits", "CPU %", "Type", "ResourceID", "Launched", "Region"}
}

// Fetch returns a channel where it will asynchronously will send Resources
func Fetch(region, account, roleARN string) chan core.Resource {
	resources := make(chan core.Resource)
	go func() {
		sess, config := core.NewConfig(region, roleARN)
		svc := rds.New(sess, config)
		resp, err := svc.DescribeDBInstances(nil)
		if err != nil {
			fmt.Printf("rds.describeRDSes %s %s %v\n", roleARN, region, err)
			return
		}
		for _, db := range resp.DBInstances {
			resources <- newRDS(db, sess, config)
		}
		close(resources)
	}()
	return resources
}

func newRDS(db *rds.DBInstance, sess client.ConfigProvider, config *aws.Config) *database {
	r := &database{
		cw:           cloudwatch.New(sess, config),
		ResourceID:   *db.DBInstanceIdentifier,
		Region:       *config.Region,
		InstanceType: *db.DBInstanceClass,
		state:        *db.DBInstanceStatus,
		LaunchTime:   db.InstanceCreateTime,
		Name:         strings.Replace(*db.DBInstanceIdentifier, "-", ".", -1) + ".db",
		namespace:    aws.String("AWS/RDS"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("DBInstanceIdentifier"),
				Value: db.DBInstanceIdentifier,
			},
		},
	}
	if strings.Contains(*db.DBInstanceClass, "t2") {
		r.burstable = true
	}
	return r
}

type database struct {
	Name             string
	ResourceID       string
	LaunchTime       *time.Time
	Region           string
	InstanceType     string
	CPUUtilization   float64
	CPUCreditBalance float64

	burstable  bool
	state      string
	namespace  *string
	dimensions []*cloudwatch.Dimension
	cw         *cloudwatch.CloudWatch
}

func (d *database) Row() []string {
	launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *d.LaunchTime)
	return []string{
		d.Name,
		fmt.Sprintf("%.2f", d.CPUCreditBalance),
		fmt.Sprintf("%.2f", d.CPUUtilization),
		d.InstanceType,
		d.ResourceID,
		launchedAgo,
		d.Region,
	}
}

func (d *database) Display() bool {
	if !d.burstable || d.state != "available" {
		return false
	}
	return d.CPUCreditBalance < 10 && d.CPUCreditBalance > 0
}

func (d *database) SetMetrics() {
	if !d.burstable || d.state != "available" {
		return
	}
	d.CPUUtilization = d.getMetric("CPUUtilization")
	d.CPUCreditBalance = d.getMetric("CPUCreditBalance")
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
	if err != nil || len(result.Datapoints) == 0 {
		fmt.Printf("rds.getMetric %s %v\n", d.Region, err)
		return -1
	}
	return *result.Datapoints[0].Average
}
