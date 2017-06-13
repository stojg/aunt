package core

import (
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
)

type List struct {
	sync.RWMutex
	items       []Resource
	lastUpdated time.Time
}

func NewList() *List {
	return &List{}
}

func (l *List) Add(r Resource) {
	l.items = append(l.items, r)
	l.lastUpdated = time.Now()
}
func (l *List) Get() []Resource {
	l.RLock()
	i := l.items
	l.RUnlock()
	return i
}

func (l *List) Set(list *List) {
	l.Lock()
	l.items = list.Get()
	l.lastUpdated = time.Now()
	l.Unlock()
}

func (l *List) Ftable(w io.Writer) {
	output := tablewriter.NewWriter(w)
	if len(l.items) > 0 {
		output.SetHeader(l.items[0].Headers())
	}
	sort.Sort(l)
	for _, i := range l.items {
		output.Append(i.Row())
	}
	output.Render()
}

func (l *List) Updated() time.Time {
	return l.lastUpdated
}

func (l *List) Len() int {
	return len(l.items)
}
func (l *List) Swap(i, j int) {
	l.items[i], l.items[j] = l.items[j], l.items[i]
}

var r = regexp.MustCompile(`[^0-9]+|[0-9]+`)

func (l *List) Less(i, j int) bool {

	spliti := r.FindAllString(strings.Replace(l.items[i].Name(), " ", "", -1), -1)
	splitj := r.FindAllString(strings.Replace(l.items[j].Name(), " ", "", -1), -1)

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
	for index := 0; index < len(l.items[i].Name()) && index < len(l.items[j].Name()); index++ {
		if isNumber(l.items[i].Name()[index]) || isNumber(l.items[j].Name()[index]) {
			return isNumber(l.items[i].Name()[index])
		}
	}
	return l.items[i].Name() < l.items[j].Name()
}

func isNumber(input uint8) bool {
	return input >= '0' && input <= '9'
}
