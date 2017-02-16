package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"strings"
	"time"
)

func NewInstance(i *ec2.Instance, region *string) *Instance {
	resource := &Instance{
		ResourceID:   *i.InstanceId,
		Region:       *region,
		InstanceType: *i.InstanceType,
		State:        *i.State.Name,
		LaunchTime:   i.LaunchTime,
		Namespace:    aws.String("AWS/EC2"),
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: i.InstanceId,
			},
		},
	}

	if strings.HasPrefix(resource.InstanceType, "t2") {
		resource.Burstable = true
	}

	for _, tag := range i.Tags {
		if *tag.Key == "Name" && len(*tag.Value) > 0 {
			resource.Name = *tag.Value
			break
		}
	}
	return resource
}

func NewRDS(db *rds.DBInstance, region *string) *Instance {

	name := strings.Replace(*db.DBInstanceIdentifier, "-", ".", -1) + ".db"
	resource := &Instance{
		ResourceID:   *db.DBInstanceIdentifier,
		Region:       *region,
		InstanceType: *db.DBInstanceClass,
		State:        *db.DBInstanceStatus,
		LaunchTime:   db.InstanceCreateTime,
		Name:         name,
		Namespace:    aws.String("AWS/RDS"),
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("DBInstanceIdentifier"),
				Value: db.DBInstanceIdentifier,
			},
		},
	}
	if strings.Contains(*db.DBInstanceClass, "t2") {
		resource.Burstable = true
	}
	return resource
}

type Instance struct {
	ResourceID       string
	LaunchTime       *time.Time
	Name             string
	InstanceType     string
	Region           string
	State            string
	Burstable        bool
	CPUUtilization   float64
	CPUCreditBalance float64
	Namespace        *string                 `json:"-"`
	Dimensions       []*cloudwatch.Dimension `json:"-"`
}
