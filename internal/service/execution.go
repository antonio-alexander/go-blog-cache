package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

func getCorrelationId(req *http.Request) string {
	if correlationId := req.Header.Get("Correlation-Id"); correlationId != "" {
		return correlationId
	}
	if correlationId := req.URL.Query().Get("correlation_id"); correlationId != "" {
		return correlationId
	}
	return ""
}

func empNoFromPath(pathVariables map[string]string) (int64, error) {
	empNo := pathVariables[data.PathEmpNo]
	return strconv.ParseInt(empNo, 10, 64)
}

func handleResponse(writer http.ResponseWriter, err error, items ...any) error {
	var statusCode int
	var bytes []byte

	if err != nil {
		var e error

		switch {
		default:
			bytes, e = json.Marshal(&data.Error{
				ErrorMessage: err.Error(),
				ErrorType:    data.ErrorTypeUnknown,
			})
			if e != nil {
				return e
			}
			statusCode = data.ErrorTypeUnknown.StatusCode()
		case errors.Is(err, data.ErrNotCached),
			errors.Is(err, data.ErrNotFound),
			errors.Is(err, data.ErrUnknown):
			err, _ := err.(*data.Error)
			bytes, e = json.Marshal(err)
			if e != nil {
				return e
			}
			statusCode = err.StatusCode()
		case errors.Is(err, data.ErrNotCachedRetry):
			err, _ := err.(*data.Error)
			bytes, e = json.Marshal(err)
			if e != nil {
				return e
			}
			statusCode = err.StatusCode()
			writer.Header().Set("Retry-After", "10")
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(statusCode)
		if _, err := writer.Write(bytes); err != nil {
			return err
		}
		return nil
	}
	switch {
	default:
		bytes, err = json.Marshal(items[0])
		if err != nil {
			return err
		}
		statusCode = http.StatusOK
	case len(items) <= 0:
		statusCode = http.StatusNoContent
	}
	writer.WriteHeader(statusCode)
	if _, err := writer.Write(bytes); err != nil {
		return err
	}
	return nil
}
