package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"sync"
)

func fetchTables(region *string) chan *table {
	out := make(chan *table)
	go func() {
		sess, err := session.NewSession(&aws.Config{Region: region})
		if err != nil {
			fmt.Println(err)
			return
		}
		svc := dynamodb.New(sess)
		resp, err := svc.ListTables(nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, t := range resp.TableNames {
			tableDesc, err := svc.DescribeTable(&dynamodb.DescribeTableInput{
				TableName: t,
			})
			if err != nil {
				fmt.Println(err)
				continue
			}
			out <- newTable(tableDesc.Table, region)
		}
		close(out)
	}()
	return out
}

func merge(regions []chan *table) chan *table {
	var wg sync.WaitGroup
	out := make(chan *table)
	output := func(c chan *table) {
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

func metrics(tables chan *table) chan *table {
	out := make(chan *table)
	go func() {
		sess, err := session.NewSession()
		defer close(out)
		if err != nil {
			fmt.Println(err)
			return
		}
		for table := range tables {
			cw := cloudwatch.New(sess, &aws.Config{Region: aws.String(table.Region)})
			table.WriteThrottleEvents = table.getMetric("WriteThrottleEvents", cw)
			table.ReadThrottleEvents = table.getMetric("ReadThrottleEvents", cw)
			out <- table
		}
	}()
	return out
}

func filter(in chan *table) chan *table {
	out := make(chan *table)
	go func() {
		for i := range in {
			if i.ReadThrottleEvents > 0 || i.WriteThrottleEvents > 0 || i.Items > 10000 {
				out <- i
			}
		}
		close(out)
	}()
	return out
}
