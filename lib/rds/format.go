package rds

import (
	"encoding/json"
	"fmt"
	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
	"io"
	"time"
)

func Ftable(w io.Writer, list []*Resource) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"RDS Name", "Credits", "Type", "ResourceID", "Launched", "Region"})
	for _, i := range list {
		launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *i.LaunchTime)
		row := []string{
			i.Name,
			fmt.Sprintf("%.2f", i.CPUCreditBalance),
			i.InstanceType,
			i.ResourceID,
			launchedAgo,
			i.Region,
		}
		table.Append(row)
	}
	table.Render()
}

func Fjson(w io.Writer, list []*Resource) {
	if len(list) == 0 {
		fmt.Fprint(w, "[]")
		return
	}
	res, err := json.MarshalIndent(list, "", "\t")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%s", res)
}
