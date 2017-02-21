package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stojg/aunt/lib/dynamodb"
	"github.com/stojg/aunt/lib/ec2"
	"github.com/stojg/aunt/lib/rds"
	"net/http"
	"os"
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
		dynamodb.Ftable(os.Stdout, dynamodb.Get(regions))
		ec2.Ftable(os.Stdout, ec2.Get(regions))
		rds.Ftable(os.Stdout, rds.Get(regions))
		return
	}

	instances := ec2.NewList()
	tables := dynamodb.NewList()
	dbs := rds.NewList()
	resourceTicker := time.NewTicker(10 * time.Minute)
	go func() {
		instances.Set(ec2.Get(regions))
		for range resourceTicker.C {
			tables.Set(dynamodb.Get(regions))
			instances.Set(ec2.Get(regions))
			dbs.Set(rds.Get(regions))
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "%s", indexHTML)
	})

	http.HandleFunc("/ec2", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			ec2.Ftable(w, instances.Get())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		ec2.Fjson(w, instances.Get())
	})

	http.HandleFunc("/dynamodb", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			dynamodb.Ftable(w, tables.Get())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		dynamodb.Fjson(w, tables.Get())
	})

	http.HandleFunc("/rds", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			rds.Ftable(w, dbs.Get())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		rds.Fjson(w, dbs.Get())
	})

	fmt.Fprintf(os.Stdout, "starting webserver at port %d\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
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
		<li><a href="/ec2">EC2</a></li>
		<li><a href="/ec2?text=1">EC2 in text format</a></li>
		<li><a href="/dynamodb">DynamoDB tables</a></li>
		<li><a href="/dynamodb?text=1">DynamoDB tables in text format</a></li>
		<li><a href="/rds">RDS</a></li>
		<li><a href="/rds?text=1">RDS in text format</a></li>
	</ul>
</body>
</html>
`
