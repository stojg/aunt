package core

import (
	"encoding/json"
	"fmt"
	"io"
)

func Fjson(w io.Writer, list interface{}) {
	res, err := json.MarshalIndent(list, "", "\t")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%s", res)
}
