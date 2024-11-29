package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

func empNoFromPath(pathVariables map[string]string) (int64, error) {
	empNo := pathVariables[data.PathEmpNo]
	return strconv.ParseInt(empNo, 10, 64)
}

func getCertificates(certFile, keyFile string) (tls.Certificate, error) {
	bytesCert, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	bytesKey, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(bytesCert, bytesKey)
}

func handleResponse(writer http.ResponseWriter, err error, items ...interface{}) {
	var bytes []byte

	if err == nil {
		switch {
		default:
			bytes, err = json.MarshalIndent(items[0], "", " ")
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
			fmt.Printf("error handling response: %s", err)
			return
		}
		if _, err := writer.Write(bytes); err != nil {
			fmt.Printf("error handling response: %s", err)
		}
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := writer.Write(bytes); err != nil {
		fmt.Printf("error handling response: %s", err)
	}
}
