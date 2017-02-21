package ec2

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"sync"
)

func getResources(region *string) chan *Instance {
	resources := make(chan *Instance)
	go func() {
		describeInstances(region, resources)
		close(resources)
	}()
	return resources
}

func metrics(instances chan *Instance) chan *Instance {
	out := make(chan *Instance)
	go func() {
		for instance := range instances {
			if instance.State != "running" {
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

func lowCreditFilter(in chan *Instance, limit float64) chan *Instance {
	out := make(chan *Instance)
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

func describeInstances(region *string, resources chan *Instance) {
	sess := session.New()
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

func merge(regions []chan *Instance) chan *Instance {
	var wg sync.WaitGroup

	out := make(chan *Instance)
	output := func(c chan *Instance) {
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
