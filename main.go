package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	olo "github.com/xorpaul/sigolo"
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
		olo.LogLevel = olo.LOG_DEBUG
	}
	olo.Debug("Config loaded")

	prepare()
	olo.Debug("Cache initialized")

	go serve()
	go serveTLS()

	// prometheus metrics server
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe("127.0.0.1:2112", nil)
	olo.Info("Listening on http://127.0.0.1:2112/metrics")

}

func loadConfig(configFile string) {
	var err error

	config, err = LoadConfig(configFile)
	if err != nil {
		olo.Fatal("Could not read config %s: '%s'", configFile, err.Error())
	}

}

func prepare() {
	var err error

	cache, err = CreateCache(config.CacheFolder)

	if err != nil {
		olo.Fatal("Could not init cache: '%s'", err.Error())
	}

	client = &http.Client{
		Timeout: time.Second * 120,
	}

	promCounters = make(map[string]prometheus.Counter)
	promCounters["TOTAL_REQUESTS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_requests_total",
		Help: "The total number of requests",
	})
	promCounters["REMOTE_ERRORS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_remote_errors_total",
		Help: "The total number of remote requests that were unsuccessfull",
	})
	promCounters["REMOTE_OK"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_remote_ok_total",
		Help: "The total number of remote requests that were successfull",
	})
	promCounters["TOTAL_HTTP_REQUESTS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_http_requests_total",
		Help: "The total number of HTTP requests",
	})
	promCounters["TOTAL_HTTPS_REQUESTS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_https_requests_total",
		Help: "The total number of HTTPS requests",
	})
	promCounters["CACHE_HIT"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_hit_total",
		Help: "The total number of requests that were already cached",
	})
	promCounters["CACHE_MISS"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_miss_total",
		Help: "The total number of requests were no cache was found",
	})
	promCounters["CACHE_TOO_OLD"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_old_total",
		Help: "The total number of requests that were already cached, but the cache was too old and needed to be renewed",
	})
	promCounters["CACHE_OK"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_ok_total",
		Help: "The total number of requests that were already cached and the cache was not too old",
	})
	promCounters["CACHE_ITEM_MISSING"] = promauto.NewCounter(prometheus.CounterOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_item_missing_total",
		Help: "Cache item was known while starting the service, but was removed afterwards, this should really be 0 otherwise something is seriously wrong",
	})

	promSummaries = make(map[string]prometheus.Summary)
	promSummaries["CACHE_READ_MEMORY"] = promauto.NewSummary(prometheus.SummaryOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_read_memory_bytes",
		Help: "The total data size of requests that were served from memory cache",
	})
	promSummaries["CACHE_READ_FILE"] = promauto.NewSummary(prometheus.SummaryOpts{
		Name: config.PrometheusMetricPrefix + "pkgproxy_cache_read_file_bytes",
		Help: "The total data size of requests that were served from the file system",
	})
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	promCounters["TOTAL_REQUESTS"].Inc()
	olo.Info("Incoming request '%s' from '%s'", r.URL.Path, r.RemoteAddr)
	protocol := "http://"
	if r.TLS != nil {
		promCounters["TOTAL_HTTPS_REQUESTS"].Inc()
		protocol = "https://"
	} else {
		promCounters["TOTAL_HTTP_REQUESTS"].Inc()
	}
	cacheURL := strings.TrimLeft(r.URL.Path, "/")
	err := validateCacheURL(cacheURL)
	if err != nil {
		handleError(nil, err, w)
		return
	}
	fullUrl := protocol + cacheURL
	olo.Info("Full incoming request for '%s' from '%s'", fullUrl, r.RemoteAddr)

	requestedURLParts := strings.Split(cacheURL, "/")
	if len(requestedURLParts) > 1 {
		requestedFQDN := requestedURLParts[0]
		requestedFQDNSave := strings.ReplaceAll(requestedFQDN, ".", "_")
		requestedFQDNSave = strings.ReplaceAll(requestedFQDNSave, "-", "_")

		if _, ok := promCounters[requestedFQDN]; !ok {
			promCounters[requestedFQDN] = promauto.NewCounter(prometheus.CounterOpts{
				Name: config.PrometheusMetricPrefix + "pkgproxy_" + requestedFQDNSave + "_total",
				Help: "The total number of requests for " + requestedFQDN,
			})
		}

		promCounters[requestedFQDN].Inc()
	}

	// Cache miss -> Load data from requested URL and add to cache
	if busy, ok := cache.has(cacheURL); !ok {
		olo.Info("CACHE_MISS for requested '%s'", cacheURL)
		promCounters["CACHE_MISS"].Inc()
		defer busy.Unlock()
		response, err := GetRemote(fullUrl)
		if err != nil {
			handleError(response, err, w)
			return
		}
	} else {
		olo.Info("CACHE_HIT for requested '%s'", cacheURL)
		promCounters["CACHE_HIT"].Inc()
	}

	// The cache has definitely the data we want, so get a reader for that
	cacheResponse, err := cache.get(fullUrl)

	if err != nil {
		handleError(nil, err, w)
	} else {
		http.ServeContent(w, r, cacheURL, cacheResponse.loadedAt, cacheResponse.content)
	}
}

func GetRemote(requestedURL string) (*http.Response, error) {
	if len(config.Proxy) > 0 {
		olo.Info("GETing " + requestedURL + " with proxy " + config.Proxy)
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(config.ProxyURL)}}
	} else {
		olo.Info("GETing " + requestedURL + " without proxy")
	}

	before := time.Now()
	response, err := client.Get(requestedURL)
	duration := time.Since(before).Seconds()
	olo.Debug("GETing " + requestedURL + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if err != nil {
		return response, err
	}

	var reader io.Reader
	reader = response.Body

	if response.StatusCode == 200 {
		promCounters["REMOTE_OK"].Inc()
		cacheURL, err := removeSchemeFromURL(requestedURL)
		if err != nil {
			return response, err
		}
		err = cache.put(cacheURL, &reader, response.ContentLength)
		if err != nil {
			return response, err
		}
		defer response.Body.Close()
		return response, nil
	} else {
		promCounters["REMOTE_ERRORS"].Inc()
		return response, errors.New("GET " + requestedURL + " returned " + strconv.Itoa(response.StatusCode))
	}
}
