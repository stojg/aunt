package rds

import (
	"fmt"
	"github.com/ararog/timeago"
	"github.com/olekukonko/tablewriter"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type List struct {
	sync.RWMutex
	items []*database
}

func NewList() *List {
	return &List{
		items: make([]*database, 0),
	}
}

func (l *List) Get() []*database {
	l.RLock()
	defer l.RUnlock()
	return l.items
}

func (l *List) Set(list *List) {
	l.Lock()
	defer l.Unlock()
	l.items = list.Get()
}

func (l *List) Ftable(w io.Writer) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"RDS Name", "Credits", "Type", "ResourceID", "Launched", "Region"})
	sort.Sort(l)
	for _, i := range l.items {
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

func (l *List) Len() int {
	return len(l.items)
}
func (l *List) Swap(i, j int) {
	l.items[i], l.items[j] = l.items[j], l.items[i]
}
func (l *List) Less(i, j int) bool {

	spliti := r.FindAllString(strings.Replace(l.items[i].Name, " ", "", -1), -1)
	splitj := r.FindAllString(strings.Replace(l.items[j].Name, " ", "", -1), -1)

	for index := 0; index < len(spliti) && index < len(splitj); index++ {
		if spliti[index] != splitj[index] {
			// Both slices are numbers
			if isNumber(spliti[index][0]) && isNumber(splitj[index][0]) {
				// Remove Leading Zeroes
				stringi := strings.TrimLeft(spliti[index], "0")
				stringj := strings.TrimLeft(splitj[index], "0")
				if len(stringi) == len(stringj) {
					for indexchar := 0; indexchar < len(stringi); indexchar++ {
						if stringi[indexchar] != stringj[indexchar] {
							return stringi[indexchar] < stringj[indexchar]
						}
					}
					return len(spliti[index]) < len(splitj[index])
				}
				return len(stringi) < len(stringj)
			}
			// One of the slices is a number (we give precedence to numbers regardless of ASCII table position)
			if isNumber(spliti[index][0]) || isNumber(splitj[index][0]) {
				return isNumber(spliti[index][0])
			}
			// Both slices are not numbers
			return spliti[index] < splitj[index]
		}

	}
	// Fall back for cases where space characters have been annihliated by the replacment call
	// Here we iterate over the unmolsested string and prioritize numbers over
	for index := 0; index < len(l.items[i].Name) && index < len(l.items[j].Name); index++ {
		if isNumber(l.items[i].Name[index]) || isNumber(l.items[j].Name[index]) {
			return isNumber(l.items[i].Name[index])
		}
	}
	return l.items[i].Name < l.items[j].Name
}

var r = regexp.MustCompile(`[^0-9]+|[0-9]+`)

func isNumber(input uint8) bool {
	return input >= '0' && input <= '9'
}
