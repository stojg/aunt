package main

import (
	"encoding/json"
	"fmt"
	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
	"io"
	"time"
)

func tableWriter(w io.Writer, list []*Instance) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Name", "Credits", "Type", "ResourceID", "Launched"})
	for _, i := range list {
		launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *i.LaunchTime)
		row := []string{
			i.Name,
			fmt.Sprintf("%.2f", i.CPUCreditBalance),
			i.InstanceType,
			i.ResourceID,
			launchedAgo,
		}
		table.Append(row)
	}
	table.Render()
}

func jsonWriter(w io.Writer, list []*Instance) {
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
