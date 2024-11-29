package internal

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func GenerateId() string {
	return uuid.Must(uuid.NewRandom()).String()
}

func DoRequest(client *http.Client, uri, method string, input interface{}, v ...interface{}) ([]byte, error) {
	var byts []byte
	var err error

	switch v := input.(type) {
	default:
		if byts, err = json.Marshal(input); err != nil {
			return nil, err
		}
	case url.Values:
		uri += "?" + v.Encode()
	}
	body := bytes.NewBuffer(byts)
	request, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	default:
		byts, _ = io.ReadAll(response.Body)
		if len(byts) > 0 {
			return nil, errors.Errorf("%s: %s", response.Status, string(byts))
		}
		return nil, errors.Errorf("%s", response.Status)
	case http.StatusNoContent:
		return []byte{}, nil
	case http.StatusOK:
		bytes, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		if len(v) > 0 {
			return bytes, json.Unmarshal(bytes, v[0])
		}
		return bytes, nil
	}
}

func GetCertificate(certFile, keyFile string) (tls.Certificate, error) {
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

func GetCaCert(caCertFile string) (*x509.CertPool, error) {
	caCertPool := x509.NewCertPool()
	if caCertFile == "" {
		return caCertPool, nil
	}
	bytes, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	caCertPool.AppendCertsFromPEM(bytes)
	return caCertPool, nil
}

func GetTlsConfig(certFile, keyFile, caCertFile string) (*tls.Config, error) {
	caCertPool, err := GetCaCert(caCertFile)
	if err != nil {
		return nil, err
	}
	certificate, err := GetCertificate(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		// TLS versions below 1.2 are considered insecure
		// see https://www.rfc-editor.org/rfc/rfc7525.txt for details
		MinVersion:   tls.VersionTLS12,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{certificate},
	}, nil
}
