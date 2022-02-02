# tiny-http-proxy

Forked from https://github.com/hauke96/tiny-http-proxy.git
Simple HTTP(S) caching proxy.
Main use case is to cache remote package repositories.

# Installation
Just clone this repo and run it:

```
git clone https://github.com/xorpaul/tiny-http-proxy.git
cd tiny-http-proxy
go run *.go
```

# Configuration
All the configuration is done in a YAML config file:

| Property | Type | Description |
|:---|:---|:---|
| `port` | `string` | The port this server is listening to. |
| `cache_folder` | `string` | The folder where the cache files are stored. This folder must exist and must be writable. |
| `debug` | `bool` | When set to true, more detailed log output is printed. |
| `max_cache_item_size_in_mb` | `int` | Maximum size in MB for the in-memory cache. Larger files are only read from disk, smaller files are delivered directly from the memory. |
| `caching_rules` | `map[string]CachingRules` | Can contain regex patterns for which cache rules can be specified, see details. |
| `default_cache_ttl` | `time.Duration as string` | Default time duration to cache responses for everything else not matching a particular CachingRules regex from `caching_rules` |
| `return_cache_if_remote_fails` | `bool` | When the cache TTL expired of a requested URL and the upstream/remote does not serve the requested file anymore, we will serve the last cached version anyway, if this is set to true. Default is false |

# Config example

```
---
debug: true
skip_timestamp_log: true
enable_log_colors: true
listen_address: 0.0.0.0
listen_port: 8080
listen_ssl_port: 8443
timeout_in_s: 500
return_cache_if_remote_fails: true
ssl_private_key: ./ssl/service.key
ssl_certificate_file: ./ssl/service.pem
cache_folder: ./cache/
debug_logging: true
max_cache_item_size_in_mb: 2
default_cache_ttl: 30m
caching_rules:
  Debian Packages:
      regex: '.*\.deb$'
      ttl: 8544h # 1 year
  RPM Packages:
      regex: '.*\.rpm$'
      ttl: 8544h # 1 year
```

# Usage example for package repositories:
Original (direct)

```
# cat /etc/apt/sources.list.d/puppet7.list
deb http://apt.puppetlabs.com stretch puppet7
```
With caching proxy:

```
# cat /etc/apt/sources.list.d/puppet7-proxy.list
deb http://YOURSERVICEURL/apt.puppetlabs.com stretch puppet7
```
Example for YUM repository:
```
# cat /etc/yum.repos.d/puppet7-proxy.repo
[puppet7]
name=Puppet 7
baseurl=http://YOURSERVICEURL/yum.puppetlabs.com/puppet7/el/$releasever/$basearch/
enabled=1
gpgcheck=1
gpgkey=http://YOURSERVICEURL/yum.puppetlabs.com/RPM-GPG-KEY-puppet-20250406
```

If you need HTTPS communication than you just need to use the caching proxy with HTTPS:

```
# cat /etc/apt/sources.list.d/puppet7-proxy-with-ssl.list
deb https://YOURSERVICEURL/apt.puppetlabs.com stretch puppet7
```
Then the caching proxy will request and cache from https://apt.puppetlabs.com

With the example config from above the *.deb and *.rpm files will be only requested once from remote and then cached for 1 year.
This means only after 1 year of the last request will the proxy refresh the cached item.

For everything else the default cache TTL will be 30 minutes, which means the repository metadata files (e.g. Packages, repomd.xml, ...) will be at most 30 minutes older than the upstream file.
This will enable servers to discover new available package versions after 30 minutes.
