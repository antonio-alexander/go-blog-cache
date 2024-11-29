package client

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
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
