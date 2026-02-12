package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

func getCertificate(sslCrtFile, sslKeyFile string) (tls.Certificate, error) {
	bytesCert, err := os.ReadFile(sslCrtFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	bytesKey, err := os.ReadFile(sslKeyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	certificate, err := tls.X509KeyPair(bytesCert, bytesKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return certificate, nil
}

func getCaCert(sslCaFile string) (*x509.CertPool, error) {
	caCertPool := x509.NewCertPool()
	if sslCaFile != "" {
		bytes, err := os.ReadFile(sslCaFile)
		if err != nil {
			return nil, err
		}
		caCertPool.AppendCertsFromPEM(bytes)
	}
	return caCertPool, nil
}

func doRequest(ctx context.Context, c *http.Client, method, uri string, item any) ([]byte, int, error) {
	var contentType string
	var contentLength int
	var body io.Reader

	switch d := item.(type) {
	case []byte:
		body = bytes.NewBuffer(d)
		contentLength = len(d)
		contentType = "application/json"
	case url.Values:
		switch method {
		default:
			uri = uri + "?" + d.Encode()
		case http.MethodPut, http.MethodPost, http.MethodPatch:
			body = strings.NewReader(d.Encode())
			contentType = "application/x-www-form-urlencoded"
			contentLength = len(d.Encode())
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return nil, -1, err
	}
	request.Header.Add("Content-Type", contentType)
	request.Header.Add("Content-Length", strconv.Itoa(contentLength))
	request.Header.Add("Correlation-Id", internal.CorrelationIdFromCtx(ctx))
	response, err := c.Do(request)
	if err != nil {
		return nil, -1, err
	}
	bytes, err := io.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return nil, -1, err
	}
	switch response.StatusCode {
	default:
		var err data.Error

		if err := json.Unmarshal(bytes, &err); err != nil {
			return nil, 0, &data.Error{
				ErrorMessage: fmt.Sprintf("status code: %d; unknown error occurred:%s",
					response.StatusCode, string(bytes)),
				ErrorType: data.ErrorTypeUnknown,
			}
		}
		if response.StatusCode == http.StatusTooManyRequests {
			i, _ := strconv.Atoi(response.Header.Get("Retry-After"))
			retryAfter := time.Duration(i) * time.Second
			return nil, int(retryAfter.Seconds()), &err
		}
		return nil, -1, &err
	case http.StatusOK, http.StatusNoContent:
		return bytes, -1, nil
	}
}
