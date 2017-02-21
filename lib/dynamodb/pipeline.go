package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"sync"
)

func tables(region *string) chan *Table {
	out := make(chan *Table)

	go func() {
		svc := dynamodb.New(session.New(), &aws.Config{Region: region})
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

func merge(regions []chan *Table) chan *Table {
	var wg sync.WaitGroup

	out := make(chan *Table)
	output := func(c chan *Table) {
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

func metrics(tables chan *Table) chan *Table {
	out := make(chan *Table)
	go func() {
		for table := range tables {
			cw := cloudwatch.New(session.New(), &aws.Config{Region: aws.String(table.Region)})
			table.WriteThrottleEvents = table.getMetric("WriteThrottleEvents", cw)
			table.ReadThrottleEvents = table.getMetric("ReadThrottleEvents", cw)
			out <- table
		}
		close(out)
	}()
	return out
}

func filter(in chan *Table) chan *Table {
	out := make(chan *Table)
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
