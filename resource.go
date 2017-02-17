package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"strings"
	"time"
)

func NewInstance(i *ec2.Instance, region *string) *Resource {
	r := &Resource{
		ResourceID:   *i.InstanceId,
		Region:       *region,
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
			r.Name = *tag.Value
			break
		}
	}
	return r
}

func NewRDS(db *rds.DBInstance, region *string) *Resource {
	r := &Resource{
		ResourceID:   *db.DBInstanceIdentifier,
		Region:       *region,
		InstanceType: *db.DBInstanceClass,
		State:        *db.DBInstanceStatus,
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
		r.Burstable = true
	}
	return r
}

type Resource struct {
	ResourceID       string
	LaunchTime       *time.Time
	Name             string
	InstanceType     string
	Region           string
	State            string
	Burstable        bool
	CPUUtilization   float64
	CPUCreditBalance float64
	namespace        *string
	dimensions       []*cloudwatch.Dimension
}

func (r *Resource) ID() string {
	return r.ResourceID
}

func (r *Resource) Namespace() *string {
	return r.namespace
}

func (r *Resource) Dimensions() []*cloudwatch.Dimension {
	return r.dimensions
}
