package main

import (
	"crypto/tls"
	"net/http"
	"strconv"
	"time"

	olo "github.com/xorpaul/sigolo"
)

// serve starts the HTTPS server with the configured SSL key and certificate
func serveTLS() {
	// TLS stuff
	tlsConfig := &tls.Config{}
	//Use only TLS v1.2
	tlsConfig.MinVersion = tls.VersionTLS12

	server := &http.Server{
		Addr:         config.ListenAddress + ":" + strconv.Itoa(config.ListenSSLPort),
		TLSConfig:    tlsConfig,
		WriteTimeout: time.Duration(config.Timeout) * time.Second,
		ReadTimeout:  time.Duration(config.Timeout) * time.Second,
		IdleTimeout:  time.Duration(config.Timeout) * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	olo.Info("Listening on https://" + config.ListenAddress + ":" + strconv.Itoa(config.ListenSSLPort) + "/")
	err := server.ListenAndServeTLS(config.CertificateFile, config.PrivateKey)
	if err != nil {
		m := "Error while trying to serve HTTPS with ssl_certificate_file " + config.CertificateFile + " and ssl_private_key " + config.PrivateKey + " " + err.Error()
		olo.Fatal(m)
	}
}

// serve starts the HTTP server
func serve() {

	server := &http.Server{
		Addr:         config.ListenAddress + ":" + strconv.Itoa(config.ListenPort),
		WriteTimeout: time.Duration(config.Timeout) * time.Second,
		ReadTimeout:  time.Duration(config.Timeout) * time.Second,
		IdleTimeout:  time.Duration(config.Timeout) * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	olo.Info("Listening on http://" + config.ListenAddress + ":" + strconv.Itoa(config.ListenPort) + "/")
	err := server.ListenAndServe()
	if err != nil {
		olo.Fatal(err.Error())
	}

}
