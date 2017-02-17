package main

import (
	"sort"
	"sync"
	"flag"
	"os"
	"github.com/aws/aws-sdk-go/aws"
	"fmt"
	"net/http"
	"time"
)

func main() {

	var port = flag.Int("port", 0, "port number to listen on")
	flag.Parse()

	regions := []*string{
		//aws.String("us-east-1"),
		//aws.String("us-west-2"),
		//aws.String("us-west-1"),
		aws.String("eu-west-1"),
		//aws.String("eu-central-1"),
		//aws.String("ap-southeast-1"),
		//aws.String("ap-northeast-1"),
		//aws.String("ap-southeast-2"),
		//aws.String("ap-northeast-2"),
		//aws.String("ap-south-1"),
		//aws.String("sa-east-1"),
	}

	if *port == 0 {
		list := updateList(regions)
		fmt.Printf("querying low cpu credit data for %d regions\n", len(regions))
		tableWriter(os.Stdout, natCaseSort(list))
		return
	}

	var instancesLock sync.RWMutex
	var instances []*Instance

	ticker := time.NewTicker(time.Second * 10 * 60)
	go func() {
		list := updateList(regions)
		instancesLock.Lock()
		instances = list
		instancesLock.Unlock()
		for t := range ticker.C {
			fmt.Printf("%s ", t)
			list := updateList(regions)
			instancesLock.Lock()
			instances = list
			instancesLock.Unlock()
		}
	}()


	http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "%s", indexHTML)

	})

	http.HandleFunc("/credits", func (w http.ResponseWriter, r *http.Request) {

		instancesLock.RLock()
		defer instancesLock.RUnlock()
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			tableWriter(w, natCaseSort(instances))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		jsonWriter(w, instances)
	})

	fmt.Printf("starting webserver at port %d\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func updateList(regions []*string) []*Instance{
	fmt.Printf("querying low cpu credit data for %d regions\n", len(regions))
	list := getList(lowCreditFilter(update(regions)))
	fmt.Println("done")
	return list
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

func lowCreditFilter(in chan *Instance) chan *Instance {
	out := make(chan *Instance)
	go func() {
		for i := range in {
			if i.CPUCreditBalance < 10.0 {
				out <- i
			}
		}
		close(out)
	}()
	return out
}

func natCaseSort(list []*Instance) []*Instance {
	newList := make([]*Instance, len(list))
	for i := range list {
		newList[i] = list[i]
	}
	if len(newList) > 1 {
		sort.Sort(InstanceSort(newList))
	}
	return newList
}

func getList(instances chan *Instance) []*Instance {
	list := make([]*Instance, 0)
	for i := range instances {
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
	</ul>
</body>
</html>
`
