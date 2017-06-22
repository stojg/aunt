package core

import (
	"io"
	"sort"
	"sync"
	"time"

	"encoding/json"
	"fmt"

	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
)

type fetchFunc func(region, account, roleARN string) chan Resource

type List struct {
	headers     []string
	fetcher     fetchFunc
	lastUpdated time.Time

	sync.RWMutex
	items []Resource
}

func NewList(headers []string, f fetchFunc) *List {
	return &List{
		headers: headers,
		fetcher: f,
	}
}

func (l *List) Update(roles map[string]string, regions []string) *List {
	resourceChans := make([]chan Resource, 0)
	for account, roleARN := range roles {
		for _, region := range regions {
			time.Sleep(time.Second)
			resourceChans = append(resourceChans, l.fetcher(region, account, roleARN))
		}
	}

	var resources []Resource
	for i := range Filter(Metrics(Merge(resourceChans))) {
		resources = append(resources, i)
	}
	l.Lock()
	l.items = resources
	l.lastUpdated = time.Now()
	l.Unlock()
	return l
}

func (l *List) Len() int {
	return len(l.items)
}

func (l *List) Updated() string {
	ago, _ := timeago.TimeAgoWithTime(time.Now(), l.lastUpdated)
	return ago
}

func (l *List) Fjson(w io.Writer) {
	l.Lock()
	res, err := json.MarshalIndent(l.items, "", "\t")
	l.Unlock()
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%s", res)
}

func (l *List) Ftable(w io.Writer) {
	output := tablewriter.NewWriter(w)
	output.SetHeader(l.headers)
	l.Lock()
	var rows [][]string
	for _, i := range l.items {
		rows = append(rows, i.Row())
	}

	sort.Sort(sortRows(rows))

	output.AppendBulk(rows)
	l.Unlock()
	output.Render()
}

type sortRows [][]string

func (l sortRows) Len() int           { return len(l) }
func (l sortRows) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l sortRows) Less(i, j int) bool { return l[i][0] < l[j][0] }
