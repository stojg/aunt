package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stojg/aunt/lib/core"
	"github.com/stojg/aunt/lib/dynamodb"
	"github.com/stojg/aunt/lib/ec2"
	"github.com/stojg/aunt/lib/rds"
	"github.com/urfave/cli"
)

var (
	Version  string
	Compiled string
)

var regions = []*string{
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

var roles = []string{}

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
				dynamodb.Get(regions, roles).Ftable(os.Stdout)
				return nil
			},
		},
		{
			Name:  "ec2",
			Usage: "show EC2 statistics",
			Action: func(c *cli.Context) error {
				ec2.Get(regions, roles).Ftable(os.Stdout)
				return nil
			},
		},
		{
			Name:  "rds",
			Usage: "show RDS statistics",
			Action: func(c *cli.Context) error {
				rds.Get(regions, roles).Ftable(os.Stdout)
				return nil
			},
		},
		{
			Name:  "serve",
			Usage: "run as a HTTP server",
			Flags: []cli.Flag{
				cli.IntFlag{Name: "port", Value: 8080},
				cli.IntFlag{Name: "refresh", Value: 10},
			},
			Action: serve,
		},
	}
	app.Run(os.Args)
}

func serve(c *cli.Context) error {
	instances := core.NewList()
	tables := core.NewList()
	dbs := core.NewList()
	resourceTicker := time.NewTicker(time.Duration(c.Int("refresh")) * time.Minute)
	go func() {
		tables.Set(dynamodb.Get(regions, roles))
		instances.Set(ec2.Get(regions, roles))
		dbs.Set(rds.Get(regions, roles))
		for range resourceTicker.C {
			tables.Set(dynamodb.Get(regions, roles))
			instances.Set(ec2.Get(regions, roles))
			dbs.Set(rds.Get(regions, roles))
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, indexHTML, instances.Len(), instances.Updated(), tables.Len(), tables.Updated(), dbs.Len(), dbs.Updated())
	})

	http.HandleFunc("/ec2", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			instances.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		core.Fjson(w, instances.Get())
	})

	http.HandleFunc("/dynamodb", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			tables.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		core.Fjson(w, tables.Get())
	})

	http.HandleFunc("/rds", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			dbs.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		core.Fjson(w, dbs.Get())
	})

	fmt.Fprintf(os.Stdout, "starting webserver at port %d\n", c.Int("port"))
	err := http.ListenAndServe(fmt.Sprintf(":%d", c.Int("port")), nil)

	if err != nil {
		return err
	}
	return nil

}

const indexHTML = `<html lang="en">
	<head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>aunt</title>
		<link rel="stylesheet"
			href="https://unpkg.com/purecss@0.6.2/build/pure-min.css"
			integrity="sha384-UQiGfs9ICog+LwheBSRCt1o5cbyKIHbwjWscjemyBMT9YCUMZffs6UqUTd0hObXD"
			crossorigin="anonymous">
		<style>
			body {
				font-family: Georgia, Times, "Times New Roman", serif;
				margin: 20px;
			}
		</style>
	</head>
	<body>
		<h1>aunt</h1>

		<p>
			Updates every 10 minutes
		</p>
		<table class="pure-table">
			<thead>
				<tr>
					<th>Type</th>
					<th>Count</th>
					<th>Human</th>
					<th>json</th>
					<th>Updated</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<td>EC2</td>
					<td>%d</td>
					<td><a href="/ec2?text=1">human</a></td>
					<td><a href="/ec2">json</a></td>
					<td>%s</td>
				</tr>
				<tr>
					<td>DynamoDB</td>
					<td>%d</td>
					<td><a href="/dynamodb?text=1">human</a></td>
					<td><a href="/dynamodb">json</a></td>
					<td>%s</td>
				</tr>
				<tr>
					<td>RDS</td>
					<td>%d</td>
					<td><a href="/rds?text=1">human</a></td>
					<td><a href="/rds">json</a></td>
					<td>%s</td>
				</tr>
			</tbody>
		</table>
	</body>
</html>
`
