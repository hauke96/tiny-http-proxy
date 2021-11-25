package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hauke96/sigolo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	debug        bool
	verbose      bool
	buildtime    string
	buildversion string

	config *Config
	cache  *Cache

	client *http.Client

	promCounters  map[string]prometheus.Counter
	promSummaries map[string]prometheus.Summary
)

func main() {

	var (
		configFileFlag = flag.String("config", "example.yaml", "which config file to use")
		versionFlag    = flag.Bool("version", false, "show build time and version number")
	)
	flag.BoolVar(&debug, "debug", false, "log debug output, defaults to false")
	flag.BoolVar(&verbose, "verbose", false, "log verbose output, defaults to false")
	flag.Parse()

	configFile := *configFileFlag
	version := *versionFlag

	if version {
		fmt.Println("tiny-http-proxy", buildversion, " Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	loadConfig(configFile)

	if config.Debug || debug {
		sigolo.LogLevel = sigolo.LOG_DEBUG
	}
	sigolo.Debug("Config loaded")

	prepare()
	sigolo.Debug("Cache initialized")

	go serve()
	go serveTLS()

	// prometheus metrics server
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe("127.0.0.1:2112", nil)
	sigolo.Info("Listening on http://127.0.0.1:2112/metrics")

}

func loadConfig(configFile string) {
	var err error

	config, err = LoadConfig(configFile)
	if err != nil {
		sigolo.Fatal("Could not read config %s: '%s'", configFile, err.Error())
	}

}

func prepare() {
	var err error

	cache, err = CreateCache(config.CacheFolder)

	if err != nil {
		sigolo.Fatal("Could not init cache: '%s'", err.Error())
	}

	client = &http.Client{
		Timeout: time.Second * 120,
	}

	promCounters = make(map[string]prometheus.Counter)
	promCounters["TOTAL_REQUESTS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_requests_total",
		Help: "The total number of requests",
	})
	promCounters["REMOTE_ERRORS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_remote_errors_total",
		Help: "The total number of remote requests that were unsuccessfull",
	})
	promCounters["REMOTE_OK"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_remote_ok_total",
		Help: "The total number of remote requests that were successfull",
	})
	promCounters["TOTAL_HTTP_REQUESTS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_http_requests_total",
		Help: "The total number of HTTP requests",
	})
	promCounters["TOTAL_HTTPS_REQUESTS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_https_requests_total",
		Help: "The total number of HTTPS requests",
	})
	promCounters["CACHE_HIT"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_cache_hit_total",
		Help: "The total number of requests that were already cached",
	})
	promCounters["CACHE_MISS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_cache_miss_total",
		Help: "The total number of requests were no cache was found",
	})
	promCounters["CACHE_TOO_OLD"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_cache_old_total",
		Help: "The total number of requests that were already cached, but the cache was too old and needed to be renewed",
	})
	promCounters["CACHE_OK"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: "itohi_pkgproxy_cache_ok_total",
		Help: "The total number of requests that were already cached and the cache was not too old",
	})

	promSummaries = make(map[string]prometheus.Summary)
	promSummaries["CACHE_READ_MEMORY"] = promauto.NewSummary(prometheus.SummaryOpts{
		Name: "itohi_pkgproxy_cache_read_memory_bytes",
		Help: "The total data size of requests that were served from memory cache",
	})
	promSummaries["CACHE_READ_FILE"] = promauto.NewSummary(prometheus.SummaryOpts{
		Name: "itohi_pkgproxy_cache_read_file_bytes",
		Help: "The total data size of requests that were served from the file system",
	})
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	promCounters["TOTAL_REQUESTS"].Inc()
	protocol := "http://"
	if r.TLS != nil {
		promCounters["TOTAL_HTTPS_REQUESTS"].Inc()
		protocol = "https://"
	} else {
		promCounters["TOTAL_HTTP_REQUESTS"].Inc()
	}
	cacheURL := strings.TrimLeft(r.URL.Path, "/")
	fullUrl := protocol + cacheURL

	requestedURLParts := strings.Split(cacheURL, "/")
	if len(requestedURLParts) > 1 {
		requestedFQDN := requestedURLParts[0]
		requestedFQDNSave := strings.ReplaceAll(requestedFQDN, ".", "_")

		if _, ok := promCounters[requestedFQDN]; !ok {
			promCounters[requestedFQDN] = promauto.NewCounter(prometheus.CounterOpts{
				Name: "itohi_pkgproxy_" + requestedFQDNSave + "_total",
				Help: "The total number of requests for " + requestedFQDN,
			})
		}

		promCounters[requestedFQDN].Inc()
	}

	sigolo.Info("Requested '%s'", fullUrl)

	// Cache miss -> Load data from requested URL and add to cache
	if busy, ok := cache.has(cacheURL); !ok {
		sigolo.Info("CACHE_MISS for requested '%s'", cacheURL)
		promCounters["CACHE_MISS"].Inc()
		defer busy.Unlock()
		err := GetRemote(fullUrl)
		if err != nil {
			handleError(err, w)
			return
		}
	} else {
		sigolo.Info("CACHE_HIT for requested '%s'", cacheURL)
		promCounters["CACHE_HIT"].Inc()
	}

	// The cache has definitely the data we want, so get a reader for that
	content, err := cache.get(fullUrl)

	if err != nil {
		handleError(err, w)
	} else {
		_, err := io.Copy(w, *content)
		if err != nil {
			handleError(err, w)
			return
		}
	}
}

func GetRemote(requestedURL string) error {

	if len(config.Proxy) > 0 {
		sigolo.Info("GETing " + requestedURL + " with proxy " + config.Proxy)
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(config.ProxyURL)}}
	} else {
		sigolo.Info("GETing " + requestedURL + " without proxy")
	}

	before := time.Now()
	response, err := client.Get(requestedURL)
	duration := time.Since(before).Seconds()
	sigolo.Debug("GETing " + requestedURL + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if err != nil {
		return err
	}

	var reader io.Reader
	reader = response.Body

	if response.StatusCode == 200 {
		promCounters["REMOTE_OK"].Inc()
		cacheURL, err := removeSchemeFromURL(requestedURL)
		if err != nil {
			return err
		}
		err = cache.put(cacheURL, &reader, response.ContentLength)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		return nil
	} else {
		promCounters["REMOTE_ERRORS"].Inc()
		return errors.New("Remote return status code " + string(response.StatusCode))
	}
}

func handleError(err error, w http.ResponseWriter) {
	sigolo.Error(err.Error())
	w.WriteHeader(500)
	fmt.Fprintf(w, err.Error())
}

// serve starts the HTTPS server with the configured SSL key and certificate
func serveTLS() {
	// TLS stuff
	tlsConfig := &tls.Config{}
	//Use only TLS v1.2
	tlsConfig.MinVersion = tls.VersionTLS12

	server := &http.Server{
		Addr:         config.ListenAddress + ":" + strconv.Itoa(config.ListenSSLPort),
		TLSConfig:    tlsConfig,
		WriteTimeout: 65 * time.Second,
		ReadTimeout:  65 * time.Second,
		IdleTimeout:  65 * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	sigolo.Info("Listening on https://" + config.ListenAddress + ":" + strconv.Itoa(config.ListenSSLPort) + "/")
	err := server.ListenAndServeTLS(config.CertificateFile, config.PrivateKey)
	if err != nil {
		m := "Error while trying to serve HTTPS with ssl_certificate_file " + config.CertificateFile + " and ssl_private_key " + config.PrivateKey + " " + err.Error()
		sigolo.Fatal(m)
	}
}

// serve starts the HTTP server
func serve() {

	server := &http.Server{
		Addr:         config.ListenAddress + ":" + strconv.Itoa(config.ListenPort),
		WriteTimeout: 65 * time.Second,
		ReadTimeout:  65 * time.Second,
		IdleTimeout:  65 * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	sigolo.Info("Listening on http://" + config.ListenAddress + ":" + strconv.Itoa(config.ListenPort) + "/")
	err := server.ListenAndServe()
	if err != nil {
		sigolo.Fatal(err.Error())
	}

}

func removeSchemeFromURL(requestedURL string) (string, error) {
	url, err := url.Parse(requestedURL)
	if err != nil {
		return "", fmt.Errorf("unable to remove URL scheme from requested URL '%s'", requestedURL)
	}
	return strings.TrimLeft(requestedURL, url.Scheme+"://"), nil
}
