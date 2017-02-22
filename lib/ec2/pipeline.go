package ec2

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"sync"
)

func fetchInstances(region *string) chan *instance {
	resources := make(chan *instance)
	go func() {
		describeInstances(region, resources)
		close(resources)
	}()
	return resources
}

func metrics(instances chan *instance) chan *instance {
	out := make(chan *instance)
	go func() {
		sess, err := session.NewSession()
		defer close(out)
		if err != nil {
			fmt.Println(err)
			return
		}
		for instance := range instances {
			if instance.State != "running" {
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

func filter(in chan *instance, limit float64) chan *instance {
	out := make(chan *instance)
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

func merge(regions []chan *instance) chan *instance {
	var wg sync.WaitGroup
	out := make(chan *instance)
	output := func(c chan *instance) {
		for instance := range c {
			out <- instance
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

func describeInstances(region *string, resources chan *instance) {
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println(err)
		return
	}
	svc := ec2.New(sess, &aws.Config{Region: region})
	resp, err := svc.DescribeInstances(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for idx := range resp.Reservations {
		for _, ec2Inst := range resp.Reservations[idx].Instances {
			resources <- newInstance(ec2Inst, region)
		}
	}
}
