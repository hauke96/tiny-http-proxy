package main

import (
	"io/ioutil"
	"net/url"
	"path/filepath"
	"time"

	olo "github.com/xorpaul/sigolo"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Debug                    bool                    `yaml:"debug"`
	SkipTimestampLog         bool                    `yaml:"skip_timestamp_log"`
	EnableColors             bool                    `yaml:"enable_log_colors"`
	ListenAddress            string                  `yaml:"listen_address"`
	ListenPort               int                     `yaml:"listen_port"`
	ListenSSLPort            int                     `yaml:"listen_ssl_port"`
	Timeout                  int                     `yaml:"timeout_in_s"`
	PrivateKey               string                  `yaml:"ssl_private_key"`
	CertificateFile          string                  `yaml:"ssl_certificate_file"`
	Proxy                    string                  `yaml:"proxy"`
	ProxyURL                 *url.URL                `yaml:"proxyURL"`
	CacheFolder              string                  `yaml:"cache_folder"`
	DefaultCacheTTLString    string                  `yaml:"default_cache_ttl"`
	DefaultCacheTTL          time.Duration           `yaml:"default_cache_ttlDuration"`
	MaxCacheItemSize         int64                   `yaml:"max_cache_item_size_in_mb"`
	CacheRules               map[string]CachingRules `yaml:"caching_rules"`
	ReturnCacheIfRemoteFails bool                    `yaml:"return_cache_if_remote_fails"`
	PrometheusMetricPrefix   string                  `yaml:"prometheus_metric_prefix"`
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

	config.ProxyURL, err = url.Parse(config.Proxy)
	if err != nil {
		return nil, err
	}

	// make sure we have the absolute path for the cache dir
	config.CacheFolder, err = filepath.Abs(config.CacheFolder)
	if err != nil {
		return nil, err
	}

	olo.Debug("using config settings: %+v", config)

	return &config, nil
}
