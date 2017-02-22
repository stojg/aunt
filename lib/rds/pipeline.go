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

func metrics(in chan *database) chan *database {
	out := make(chan *database)
	go func() {
		sess, err := session.NewSession()
		defer close(out)
		if err != nil {
			fmt.Println(err)
			return
		}
		for d := range in {
			if d.State != "available" {
				continue
			}
			if !d.Burstable {
				continue
			}
			cw := cloudwatch.New(sess, &aws.Config{Region: aws.String(d.Region)})
			d.CPUUtilization = d.getMetric("CPUUtilization", cw)
			d.CPUCreditBalance = d.getMetric("CPUCreditBalance", cw)
			out <- d
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

func merge(in []chan *database) chan *database {
	var wg sync.WaitGroup
	out := make(chan *database)
	output := func(c chan *database) {
		for table := range c {
			out <- table
		}
		wg.Done()
	}
	wg.Add(len(in))
	for _, c := range in {
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
