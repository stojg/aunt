package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

func main() {

	var port = flag.Int("port", 0, "port number to listen on")
	flag.Parse()

	regions := []*string{
		aws.String("us-east-1"),
		aws.String("us-west-2"),
		aws.String("us-west-1"),
		aws.String("eu-west-1"),
		aws.String("eu-central-1"),
		aws.String("ap-southeast-1"),
		aws.String("ap-northeast-1"),
		aws.String("ap-southeast-2"),
		aws.String("ap-northeast-2"),
		aws.String("ap-south-1"),
		aws.String("sa-east-1"),
	}

	if *port == 0 {
		list := getResourceList(regions)
		fmt.Printf("querying low cpu credit data for %d regions\n", len(regions))
		resourceTableWriter(os.Stdout, natCaseSort(list))
		dynamoDBTableWriter(os.Stdout, getDynamoDBList(regions))
		return
	}

	var instancesLock sync.RWMutex
	var instances []*Resource
	resourceTicker := time.NewTicker(time.Second * 10 * 60)
	go func() {
		list := getResourceList(regions)
		instancesLock.Lock()
		instances = list
		instancesLock.Unlock()
		for t := range resourceTicker.C {
			fmt.Printf("%s ", t)
			list := getResourceList(regions)
			instancesLock.Lock()
			instances = list
			instancesLock.Unlock()
		}
	}()

	var dynamoDBLock sync.RWMutex
	var dynamoTables []*Dynamodb
	dynamoDBTicker := time.NewTicker(time.Second * 10 * 60)
	go func() {
		list := getDynamoDBList(regions)
		dynamoDBLock.Lock()
		dynamoTables = list
		dynamoDBLock.Unlock()
		for t := range dynamoDBTicker.C {
			fmt.Printf("%s ", t)
			list := getDynamoDBList(regions)
			dynamoDBLock.Lock()
			dynamoTables = list
			dynamoDBLock.Unlock()
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "%s", indexHTML)
	})

	http.HandleFunc("/credits", func(w http.ResponseWriter, r *http.Request) {
		instancesLock.RLock()
		defer instancesLock.RUnlock()
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			resourceTableWriter(w, natCaseSort(instances))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resourceJsonWriter(w, instances)
	})

	http.HandleFunc("/dynamo", func(w http.ResponseWriter, r *http.Request) {
		dynamoDBLock.RLock()
		defer dynamoDBLock.RUnlock()
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			dynamoDBTableWriter(w, dynamoTables)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		dynamoDBJsonWriter(w, dynamoTables)
	})

	fmt.Printf("starting webserver at port %d\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func getDynamoDBList(regions []*string) []*Dynamodb {
	fmt.Printf("querying dynamodb data in %d regions\n", len(regions))
	list := drainDynamoDB(throttleFilter(fetchDynamoDBs(regions)))

	fmt.Println("done")
	return list
}

func getResourceList(regions []*string) []*Resource {
	fmt.Printf("querying low cpu credit data in %d regions\n", len(regions))
	list := drainResources(lowCreditFilter(fetchResources(regions), 10.0))
	fmt.Println("done")
	return list
}

func mergeResources(regions []chan *Resource) chan *Resource {
	var wg sync.WaitGroup

	out := make(chan *Resource)
	output := func(c chan *Resource) {
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

func mergeDynamoDBs(regions []chan *Dynamodb) chan *Dynamodb {
	var wg sync.WaitGroup

	out := make(chan *Dynamodb)
	output := func(c chan *Dynamodb) {
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

func throttleFilter(in chan *Dynamodb) chan *Dynamodb {
	out := make(chan *Dynamodb)
	go func() {
		for i := range in {
			if i.ReadThrottleEvents > 0 || i.WriteThrottleEvents > 0 {
				out <- i
			}
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

func natCaseSort(list []*Resource) []*Resource {
	newList := make([]*Resource, len(list))
	for i := range list {
		newList[i] = list[i]
	}
	if len(newList) > 1 {
		sort.Sort(InstanceSort(newList))
	}
	return newList
}

func drainResources(instances chan *Resource) []*Resource {
	list := make([]*Resource, 0)
	for i := range instances {
		list = append(list, i)
	}
	return list
}

func drainDynamoDB(tables chan *Dynamodb) []*Dynamodb {
	list := make([]*Dynamodb, 0)
	for i := range tables {
		list = append(list, i)
	}
	return list
}

const indexHTML = `<!doctype html>
<html lang="en-gb">
<head>
	<title>aunt</title>
	<link rel="stylesheet" href="https://raw.githubusercontent.com/necolas/normalize.css/master/normalize.css">
</head>
<body>
	<h1>aunt</h1>
	<ul>
		<li><a href="/credits">credits</a></li>
		<li><a href="/credits?text=1">credits in text format</a></li>
		<li><a href="/dynamo">throttled dynamodb tables</a></li>
		<li><a href="/dynamo?text=1"> throttled dynamo tables in text format</a></li>
	</ul>
</body>
</html>
`
