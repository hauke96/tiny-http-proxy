package main

import (
	"errors"
	"io/ioutil"
	"net"
	"net/url"
	"path/filepath"
	"time"

	h "github.com/xorpaul/gohelper"
	olo "github.com/xorpaul/sigolo"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Debug                      bool     `yaml:"debug"`
	SkipTimestampLog           bool     `yaml:"skip_timestamp_log"`
	EnableColors               bool     `yaml:"enable_log_colors"`
	ListenAddress              string   `yaml:"listen_address"`
	ListenPort                 int      `yaml:"listen_port"`
	ListenSSLPort              int      `yaml:"listen_ssl_port"`
	Timeout                    int      `yaml:"timeout_in_s"`
	ProxyNetworkStrings        []string `yaml:"reverse_proxy_networks"`
	ProxyNetworks              []net.IPNet
	PrivateKey                 string                  `yaml:"ssl_private_key"`
	CertificateFile            string                  `yaml:"ssl_certificate_file"`
	Proxy                      string                  `yaml:"proxy"`
	ProxyURL                   *url.URL                `yaml:"proxyURL"`
	PrefillCacheOnStartup      bool                    `yaml:"prefill_cache_on_startup"`
	CacheFolder                string                  `yaml:"cache_folder"`
	CacheFolderHTTPS           string                  `yaml:"cache_folder_https"`
	DefaultCacheTTLString      string                  `yaml:"default_cache_ttl"`
	DefaultCacheTTL            time.Duration           `yaml:"default_cache_ttlDuration"`
	ServiceNameDefaultCacheTTL map[string]CachingRules `yaml:"service_default_cache_ttl"`
	MaxCacheItemSize           int64                   `yaml:"max_cache_item_size_in_mb"`
	CacheRules                 map[string]CachingRules `yaml:"caching_rules"`
	ReturnCacheIfRemoteFails   bool                    `yaml:"return_cache_if_remote_fails"`
	PrometheusMetricPrefix     string                  `yaml:"prometheus_metric_prefix"`
}

type CachingRules struct {
	Regex     string        `yaml:"regex"`
	TTLString string        `yaml:"ttl"`
	TTL       time.Duration `yaml:"ttlDuration"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(file, &config)

	if err != nil {
		return nil, err
	}

	if config.SkipTimestampLog {
		olo.SkipTimestamp = true
	}

	if config.EnableColors {
		olo.EnableColors = true
	}

	config.ProxyNetworks, err = h.ParseNetworks(config.ProxyNetworkStrings, "in reverse_proxy_networks")
	if err != nil {
		return nil, err
	}

	for name, cr := range config.CacheRules {
		olo.Info("adding caching rule '%s': regex:'%s' ttl:'%s'", name, cr.Regex, cr.TTLString)
		cr.TTL, err = time.ParseDuration(cr.TTLString)
		if err != nil {
			return nil, err
		}
		olo.Info("setting ttl to '%s' for regex '%s'", cr.TTL, cr.Regex)
		config.CacheRules[name] = cr
	}

	config.DefaultCacheTTL, err = time.ParseDuration(config.DefaultCacheTTLString)
	if err != nil {
		return nil, err
	}

	for name, cr := range config.ServiceNameDefaultCacheTTL {
		olo.Info("adding default caching rule for service name '%s': ttl:'%s'", name, cr.TTLString)
		cr.TTL, err = time.ParseDuration(cr.TTLString)
		if err != nil {
			return nil, err
		}
		config.ServiceNameDefaultCacheTTL[name] = cr
	}

	config.ProxyURL, err = url.Parse(config.Proxy)
	if err != nil {
		return nil, err
	}

	// make sure we have the absolute path for the cache dir
	config.CacheFolder, err = filepath.Abs(config.CacheFolder)
	if err != nil {
		return nil, err
	}
	if len(config.CacheFolder) == 0 {
		return nil, errors.New("config.CacheFolder setting is not found in config " + path)
	}
	if len(config.CacheFolderHTTPS) == 0 {
		return nil, errors.New("config.CacheFolderHTTPS setting is not found in config " + path)
	}

	config.CacheFolderHTTPS, err = filepath.Abs(config.CacheFolderHTTPS)
	if err != nil {
		return nil, err
	}

	olo.Debug("using config settings: %+v", config)

	return &config, nil
}
