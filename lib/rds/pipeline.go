package rds

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/rds"
	"sync"
)

func fetchDatabases(region *string) chan *database {
	resources := make(chan *database)
	go func() {
		describeRDSes(region, resources)
		close(resources)
	}()
	return resources
}

func metrics(instances chan *database) chan *database {
	out := make(chan *database)
	go func() {
		sess, err := session.NewSession()
		defer close(out)
		if err != nil {
			fmt.Println(err)
			return
		}
		for instance := range instances {
			if instance.State != "available" {
				continue
			}
			if !instance.Burstable {
				continue
			}
			cw := cloudwatch.New(sess, &aws.Config{Region: aws.String(instance.Region)})
			instance.CPUUtilization = instance.getMetric("CPUUtilization", cw)
			instance.CPUCreditBalance = instance.getMetric("CPUCreditBalance", cw)
			out <- instance
		}
	}()
	return out
}

func filter(in chan *database, limit float64) chan *database {
	out := make(chan *database)
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

func merge(regions []chan *database) chan *database {
	var wg sync.WaitGroup
	out := make(chan *database)
	output := func(c chan *database) {
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

func describeRDSes(region *string, resources chan *database) {
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println(err)
		return
	}
	svc := rds.New(sess, &aws.Config{Region: region})
	resp, err := svc.DescribeDBInstances(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, db := range resp.DBInstances {
		resources <- newRDS(db, region)
	}
}
