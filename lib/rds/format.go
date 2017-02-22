package rds

import (
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
