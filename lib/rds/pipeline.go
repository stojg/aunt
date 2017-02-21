package rds

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"sync"
)

func dbs(region *string) chan *Resource {
	resources := make(chan *Resource)
	go func() {
		describeRDSes(region, resources)
		close(resources)
	}()
	return resources
}

func metrics(instances chan *Resource) chan *Resource {

	out := make(chan *Resource)

	go func() {
		for instance := range instances {
			if instance.State != "available" {
				continue
			}
			if !instance.Burstable {
				continue
			}
			cw := cloudwatch.New(session.New(), &aws.Config{Region: aws.String(instance.Region)})
			instance.CPUUtilization = instance.getMetric("CPUUtilization", cw)
			instance.CPUCreditBalance = instance.getMetric("CPUCreditBalance", cw)
			out <- instance
		}
		close(out)
	}()
	return out
}

func lowCreditFilter(in chan *Resource, limit float64) chan *Resource {
	out := make(chan *Resource)
	go func() {
		for i := range in {
			if i.CPUCreditBalance < limit {
				out <- i
			}
		}
		close(out)
	}()
	return out
}

func merge(regions []chan *Resource) chan *Resource {
	var wg sync.WaitGroup

	out := make(chan *Resource)
	output := func(c chan *Resource) {
		for table := range c {
			out <- table
		}
		wg.Done()
	}
	wg.Add(len(regions))
	for _, c := range regions {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func describeRDSes(region *string, resources chan *Resource) {
	sess := session.New()
	svc := rds.New(sess, &aws.Config{Region: region})
	resp, err := svc.DescribeDBInstances(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, db := range resp.DBInstances {
		resources <- NewRDS(db, region)
	}
}
