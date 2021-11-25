package main

import (
	"io/ioutil"
	"net/url"
	"time"

	"github.com/hauke96/sigolo"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Debug                 bool                    `yaml:"debug"`
	ListenAddress         string                  `yaml:"listen_address"`
	ListenPort            int                     `yaml:"listen_port"`
	ListenSSLPort         int                     `yaml:"listen_ssl_port"`
	PrivateKey            string                  `yaml:"ssl_private_key"`
	CertificateFile       string                  `yaml:"ssl_certificate_file"`
	Proxy                 string                  `yaml:"proxy"`
	ProxyURL              *url.URL                `yaml:"proxyURL"`
	CacheFolder           string                  `yaml:"cache_folder"`
	DefaultCacheTTLString string                  `yaml:"default_cache_ttl"`
	DefaultCacheTTL       time.Duration           `yaml:"default_cache_ttlDuration"`
	MaxCacheItemSize      int64                   `yaml:"max_cache_item_size_in_mb"`
	CacheRules            map[string]CachingRules `yaml:"caching_rules"`
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

	for name, cr := range config.CacheRules {
		sigolo.Info("adding caching rule '%s': regex:'%s' ttl:'%s'", name, cr.Regex, cr.TTLString)
		cr.TTL, err = time.ParseDuration(cr.TTLString)
		if err != nil {
			return nil, err
		}
		sigolo.Info("setting ttl to '%s' for regex '%s'", cr.TTL, cr.Regex)
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

	sigolo.Debug("using config settings: %+v", config)

	return &config, nil
}
