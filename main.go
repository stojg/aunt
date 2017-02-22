package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stojg/aunt/lib/core"
	"github.com/stojg/aunt/lib/dynamodb"
	"github.com/stojg/aunt/lib/ec2"
	"github.com/stojg/aunt/lib/rds"
	"github.com/urfave/cli"
	"net/http"
	"os"
	"time"
)

var (
	Version  string
	Compiled string
)

var regions = []*string{
	//aws.String("us-east-1"),
	aws.String("us-west-2"),
	//aws.String("us-west-1"),
	aws.String("eu-west-1"),
	//aws.String("eu-central-1"),
	//aws.String("ap-southeast-1"),
	//aws.String("ap-northeast-1"),
	aws.String("ap-southeast-2"),
	//aws.String("ap-northeast-2"),
	//aws.String("ap-south-1"),
	//aws.String("sa-east-1"),
}

func main() {

	app := cli.NewApp()
	app.Version = Version
	cParsed, _ := time.Parse(time.RFC1123, Compiled)
	app.Compiled = cParsed

	cli.AppHelpTemplate = fmt.Sprintf(`%s
COMPILED: {{.Compiled}}
SUPPORT:  http://github.com/stojg/aunt
`, cli.AppHelpTemplate)

	app.Commands = []cli.Command{
		{
			Name:  "dynamodb",
			Usage: "show DynamoDB statistics",
			Action: func(c *cli.Context) error {
				dynamodb.Ftable(os.Stdout, dynamodb.Get(regions))
				return nil
			},
		},
		{
			Name:  "ec2",
			Usage: "show EC2 statistics",
			Action: func(c *cli.Context) error {
				ec2.Ftable(os.Stdout, ec2.Get(regions))
				return nil
			},
		},
		{
			Name:  "rds",
			Usage: "show RDS statistics",
			Action: func(c *cli.Context) error {
				rds.Ftable(os.Stdout, rds.Get(regions))
				return nil
			},
		},
		{
			Name:  "serve",
			Usage: "run as a HTTP server",
			Flags: []cli.Flag{
				cli.IntFlag{Name: "port", Value: 8080},
			},
			Action: serve,
		},
	}
	app.Run(os.Args)
}

func serve(c *cli.Context) error {
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
		core.Fjson(w, tables.Get())
	})

	http.HandleFunc("/dynamodb", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			dynamodb.Ftable(w, tables.Get())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		core.Fjson(w, tables.Get())
	})

	http.HandleFunc("/rds", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			rds.Ftable(w, dbs.Get())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		core.Fjson(w, tables.Get())
	})

	fmt.Fprintf(os.Stdout, "starting webserver at port %d\n", c.Int("port"))
	err := http.ListenAndServe(fmt.Sprintf(":%d", c.Int("port")), nil)

	if err != nil {
		return err
	}
	return nil

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
