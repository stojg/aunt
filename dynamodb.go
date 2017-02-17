package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"time"
)

func NewDynamoDB(t *dynamodb.TableDescription, region *string) *Dynamodb {
	return &Dynamodb{
		ResourceID: *t.TableName,
		LaunchTime: t.CreationDateTime,
		Name:       *t.TableName,
		Region:     *region,
		namespace:  aws.String("AWS/DynamoDB"),
		dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("TableName"),
				Value: t.TableName,
			},
		},
	}
}

type Dynamodb struct {
	ResourceID          string
	LaunchTime          *time.Time
	Name                string
	Region              string
	namespace           *string
	dimensions          []*cloudwatch.Dimension
	WriteThrottleEvents float64
	ReadThrottleEvents  float64
}

func (r *Dynamodb) ID() string {
	return r.ResourceID
}

func (r *Dynamodb) Namespace() *string {
	return r.namespace
}

func (r *Dynamodb) Dimensions() []*cloudwatch.Dimension {
	return r.dimensions
}
