package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stojg/aunt/lib/core"
	"github.com/stojg/aunt/lib/dynamodb"
	"github.com/stojg/aunt/lib/ebs"
	"github.com/stojg/aunt/lib/ec2"
	"github.com/stojg/aunt/lib/rds"
	"github.com/urfave/cli"
)

var (
	Version  string
	Compiled string
)

var regions = []string{
	"ap-southeast-2",
	"us-east-1",
	"us-west-2",
	"us-west-1",
	"eu-west-1",
	"eu-central-1",
	"ap-southeast-1",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-south-1",
}

var roles = map[string]string{}

type Config struct {
	Roles map[string]string
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
				tables := core.NewList(dynamodb.Headers(), dynamodb.Fetch)
				tables.Update(roles, regions).Ftable(os.Stdout)
				return nil
			},
		},
		{
			Name:  "ebs",
			Usage: "show EBS statistics",
			Action: func(c *cli.Context) error {
				volumes := core.NewList(ebs.Headers(), ebs.Fetch)
				volumes.Update(roles, regions).Ftable(os.Stdout)
				return nil
			},
		},
		{
			Name:  "ec2",
			Usage: "show EC2 statistics",
			Action: func(c *cli.Context) error {
				instances := core.NewList(ec2.Headers(), ec2.Fetch)
				instances.Update(roles, regions).Ftable(os.Stdout)
				return nil
			},
		},
		{
			Name:  "rds",
			Usage: "show RDS statistics",
			Action: func(c *cli.Context) error {
				databases := core.NewList(rds.Headers(), rds.Fetch)
				databases.Update(roles, regions).Ftable(os.Stdout)
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

// canConnectToAWS does a simple region less call to AWS to check if we can connect to AWS api and it has the
// benefit of recycling / updating the ec2 instance credentials that can expire if we don't occasional do a non
// assumed-role API call
func canConnectToAWS() error {
	sess := session.Must(session.NewSession())
	svc := sts.New(sess)
	_, err := svc.GetCallerIdentity(nil)
	return err
}

func serve(c *cli.Context) error {
	instances := core.NewList(ec2.Headers(), ec2.Fetch)
	tables := core.NewList(dynamodb.Headers(), dynamodb.Fetch)
	databases := core.NewList(rds.Headers(), rds.Fetch)
	volumes := core.NewList(ebs.Headers(), ebs.Fetch)

	resourceTicker := time.NewTicker(time.Duration(c.Int("refresh")) * time.Minute)
	go func() {
		fmt.Println("starting")
		if err := canConnectToAWS(); err != nil {
			fmt.Println(err)
		}
		tables.Update(roles, regions)
		instances.Update(roles, regions)
		databases.Update(roles, regions)
		volumes.Update(roles, regions)
		for range resourceTicker.C {
			if err := canConnectToAWS(); err != nil {
				fmt.Println(err)
				continue
			}
			tables.Update(roles, regions)
			instances.Update(roles, regions)
			databases.Update(roles, regions)
			volumes.Update(roles, regions)
		}
		fmt.Println("done")
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, indexHTML, c.Int("refresh"), instances.Len(), instances.Updated(), tables.Len(), tables.Updated(), databases.Len(), databases.Updated(), volumes.Len(), volumes.Updated())
	})

	http.HandleFunc("/ec2", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			instances.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		instances.Fjson(w)
	})

	http.HandleFunc("/dynamodb", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			tables.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		tables.Fjson(w)
	})

	http.HandleFunc("/rds", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			databases.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		databases.Fjson(w)
	})

	http.HandleFunc("/ebs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "" {
			w.Header().Set("Content-Type", "text/plain")
			volumes.Ftable(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		volumes.Fjson(w)
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
			Updates every %d minutes
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
				<tr>
					<td>EBS</td>
					<td>%d</td>
					<td><a href="/ebs?text=1">human</a></td>
					<td><a href="/ebs">json</a></td>
					<td>%s</td>
				</tr>
			</tbody>
		</table>
	</body>
</html>
`
