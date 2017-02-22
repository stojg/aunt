package dynamodb

import (
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
