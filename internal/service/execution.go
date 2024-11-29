package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

func empNoFromPath(pathVariables map[string]string) (int64, error) {
	empNo := pathVariables[data.PathEmpNo]
	return strconv.ParseInt(empNo, 10, 64)
}

func handleResponse(writer http.ResponseWriter, err error, items ...interface{}) {
	var bytes []byte

	if err == nil {
		switch {
		default:
			bytes, err = json.Marshal(items[0])
		case len(items) <= 0:
			writer.WriteHeader(http.StatusNoContent)
		}
	}
	if err != nil {
		var e struct {
			Error string `json:"error"`
		}

		writer.WriteHeader(http.StatusInternalServerError)
		e.Error = err.Error()
		bytes, err = json.Marshal(&e)
		if err != nil {
			fmt.Printf("error handling response: %s\n", err)
			return
		}
		if _, err := writer.Write(bytes); err != nil {
			fmt.Printf("error handling response: %s\n", err)
		}
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := writer.Write(bytes); err != nil {
		fmt.Printf("error handling response: %s\n", err)
	}
}
