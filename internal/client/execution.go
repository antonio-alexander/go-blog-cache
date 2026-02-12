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

func getCertificates(sslCrtFile, sslKeyFile string) ([]tls.Certificate, error) {
	if sslCrtFile == "" || sslKeyFile == "" {
		return []tls.Certificate{}, nil
	}
	bytesCert, err := os.ReadFile(sslCrtFile)
	if err != nil {
		return nil, err
	}
	bytesKey, err := os.ReadFile(sslKeyFile)
	if err != nil {
		return nil, err
	}
	certificate, err := tls.X509KeyPair(bytesCert, bytesKey)
	if err != nil {
		return nil, err
	}
	return []tls.Certificate{certificate}, nil
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

func getTlsConfig(sslCaFile, sslCrtFile, sslKeyFile string) (*http.Transport, error) {
	if sslCaFile == "" || sslCrtFile == "" || sslKeyFile == "" {
		return &http.Transport{}, nil
	}
	caCertPool, err := getCaCert(sslCaFile)
	if err != nil {
		return nil, err
	}
	certificates, err := getCertificates(sslCrtFile, sslKeyFile)
	if err != nil {
		return nil, err
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			// TLS versions below 1.2 are considered insecure
			// see https://www.rfc-editor.org/rfc/rfc7525.txt for details
			MinVersion:   tls.VersionTLS12,
			RootCAs:      caCertPool,
			Certificates: certificates,
		},
	}, nil
}

func doRequest(ctx context.Context, c *http.Client, method, uri string, item any) ([]byte, time.Duration, error) {
	var contentLength int
	var contentType string
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
		return nil, 0, err
	}
	request.Header.Add("Content-Type", contentType)
	request.Header.Add("Content-Length", strconv.Itoa(contentLength))
	request.Header.Add("Correlation-Id", internal.CorrelationIdFromCtx(ctx))
	response, err := c.Do(request)
	if err != nil {
		return nil, 0, err
	}
	bytes, err := io.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return nil, 0, err
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
			return nil, retryAfter, &err
		}
		return nil, 0, &err
	case http.StatusOK, http.StatusNoContent:
		return bytes, 0, nil
	}
}
