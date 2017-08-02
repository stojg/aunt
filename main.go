package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/stojg/aunt/lib/core"
	"github.com/stojg/aunt/lib/dynamodb"
	"github.com/stojg/aunt/lib/ebs"
	"github.com/stojg/aunt/lib/ec2"
	"github.com/stojg/aunt/lib/rds"

	"github.com/asdine/storm"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

var (
	// Version is used as a compile time flag
	Version string
	// Compiled is used as a compile time flag
	Compiled string
)

var regions = []string{}

var roles = map[string]string{}

// Config holds configuration data, typically loaded from a file
type Config struct {
	Roles    map[string]string
	Regions  []string
	Opsgenie struct {
		APIKey string
	}
}

func main() {

	app := cli.NewApp()
	app.Version = Version
	cParsed, err := time.Parse(time.RFC1123, Compiled)
	if err != nil {
		cParsed = time.Time{}
	}
	app.Compiled = cParsed

	cli.AppHelpTemplate = fmt.Sprintf(`%s
COMPILED: {{.Compiled}}
SUPPORT:  http://github.com/stojg/aunt
`, cli.AppHelpTemplate)

	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "config", Value: "/etc/aunt.json", Usage: "path to config file"},
	}

	app.Before = func(c *cli.Context) error {
		cfg, err := LoadConfig(c.GlobalString("config"))
		if err == nil {
			roles = cfg.Roles
			regions = cfg.Regions
			if cfg.Opsgenie.APIKey == "" {
				return nil
			}
			return core.SetOpsGenieToken(cfg.Opsgenie.APIKey)
		}
		return fmt.Errorf("error during config file read: %v", err)
	}

	db, err := storm.Open("aunt.db")
	if err != nil {
		// lazy, fix
		panic(err)
	}
	defer db.Close()

	app.Commands = []cli.Command{
		{
			Name:  "update",
			Usage: "update a metrics",
			Action: func(c *cli.Context) error {
				if err := ec2.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := rds.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := ebs.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := dynamodb.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := core.Purge(db, 15*time.Minute); err != nil {
					return fmt.Errorf("error during alert purge: %v", err)
				}
				return nil
			},
		},
		{
			Name:  "ec2",
			Usage: "EC2 metrics",
			Action: func(c *cli.Context) error {
				if err := ec2.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := ec2.Purge(db, 15*time.Minute); err != nil {
					return fmt.Errorf("error during purge: %v", err)
				}
				header, rows, err := ec2.TableData(db)
				if err != nil {
					return err
				}
				displayTable(header, rows)
				return nil
			},
		},
		{
			Name:  "rds",
			Usage: "RDS metrics",
			Action: func(c *cli.Context) error {
				if err := rds.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := rds.Purge(db, 15*time.Minute); err != nil {
					return fmt.Errorf("error during purge: %v", err)
				}
				header, rows, err := rds.TableData(db)
				if err != nil {
					return err
				}
				displayTable(header, rows)
				return nil
			},
		},
		{
			Name:  "ebs",
			Usage: "show EBS statistics",
			Action: func(c *cli.Context) error {
				if err := ebs.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := ebs.Purge(db, 15*time.Minute); err != nil {
					return fmt.Errorf("error during purge: %v", err)
				}
				header, rows, err := ebs.TableData(db)
				if err != nil {
					return err
				}
				displayTable(header, rows)
				return nil
			},
		},
		{
			Name:  "dynamodb",
			Usage: "show DynamoDB statistics",
			Action: func(c *cli.Context) error {
				if err := dynamodb.Update(db, roles, regions); err != nil {
					return fmt.Errorf("error during update: %v", err)
				}
				if err := dynamodb.Purge(db, 15*time.Minute); err != nil {
					return fmt.Errorf("error during purge: %v", err)
				}
				header, rows, err := dynamodb.TableData(db)
				if err != nil {
					return err
				}
				displayTable(header, rows)
				return nil
			},
		},

		{
			Name:  "serve",
			Usage: "run as a HTTP server",
			Flags: []cli.Flag{
				cli.IntFlag{Name: "port", Value: 8080},
			},
			Action: func(c *cli.Context) error {
				resourceTicker := time.NewTicker(10 * time.Minute)
				for {
					if err := ec2.Update(db, roles, regions); err != nil {
						fmt.Printf("error during update: %v", err)
					}
					if err := rds.Update(db, roles, regions); err != nil {
						fmt.Printf("error during update: %v", err)
					}
					if err := ebs.Update(db, roles, regions); err != nil {
						fmt.Printf("error during update: %v", err)
					}
					if err := dynamodb.Update(db, roles, regions); err != nil {
						fmt.Printf("error during update: %v", err)
					}
					if err := core.Purge(db, 15*time.Minute); err != nil {
						fmt.Printf("error during alert purge: %v", err)
					}
					<-resourceTicker.C
				}
				return nil
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("aunt: %v\n", err)
		os.Exit(1)
	}
}

func displayTable(headers []string, rows [][]string) {
	output := tablewriter.NewWriter(os.Stdout)
	output.SetHeader(headers)
	sort.Sort(core.RowSorter(rows))
	output.AppendBulk(rows)
	output.Render()
}

// LoadConfig loads and json configuration file into Config struct
func LoadConfig(file string) (*Config, error) {
	cfg := &Config{}
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
