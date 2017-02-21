package ec2

import (
	"encoding/json"
	"fmt"
	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
	"io"
	//"sort"
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

func Fjson(w io.Writer, list []*Instance) {
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

//func natCaseSort(list []*Resource) []*Resource {
//	newList := make([]*Resource, len(list))
//	for i := range list {
//		newList[i] = list[i]
//	}
//	if len(newList) > 1 {
//		sort.Sort(InstanceSort(newList))
//	}
//	return newList
//}
