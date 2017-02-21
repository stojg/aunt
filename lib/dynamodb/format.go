package dynamodb

import (
	"encoding/json"
	"fmt"
	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
	"io"
	"time"
)

func Ftable(w io.Writer, list []*Table) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"DynamoDB Name", "Throttled read events (60min)", "Throttled write events (60min)", "Entries", "Launched", "Region"})
	for _, i := range list {
		launchedAgo, _ := timeago.TimeAgoWithTime(time.Now(), *i.LaunchTime)
		row := []string{
			i.Name,
			fmt.Sprintf("%.0f", i.ReadThrottleEvents),
			fmt.Sprintf("%.0f", i.WriteThrottleEvents),
			fmt.Sprintf("%d", i.Items),
			launchedAgo,
			i.Region,
		}
		table.Append(row)
	}
	table.Render()
}

func Fjson(w io.Writer, list []*Table) {
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
