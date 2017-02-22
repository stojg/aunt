package ec2

import (
	"fmt"
	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
	"io"
	"time"
)

func Ftable(w io.Writer, list []*Instance) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Instance Name", "Credits", "Type", "ResourceID", "Launched", "Region"})
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
