# tiny-http-proxy
Maybe the tiniest HTTP proxy that also has a cache.

The tiny-http-proxy acts as a reverse proxy for one server of your choice illustrated by this picture:

```
           network            proxy             cache
         .—————————.       .—————————.       .—————————.
    <————|— — < — —|———<———|— — < — —|———<———|— < —.—. |
you ————>|— — > — —|———>———|— —.— > —|———>———|— > —' | |
         |         |       |   |(*)  |       |       | |
         |    ,—< —|———<———|< —'     |       |       | |
         |    | ,—>|———>———|— — > — —|———>———|— > ———' |
         `————+—+——´       `—————————´       `—————————´
              | |
              '—'
            website

(*) When the data is not in the cache, the website will be requested and is directly stored in the cache.
```
Where "network" may be anything (LAN/WAN/...).

# Installation
Just clone this repo and run it:

```
git clone https://github.com/hauke96/tiny-http-proxy.git
cd tiny-http-proxy
mkdir cache
go run *.go
```
Alternative to the `go run` command you can also use `make run` as well as `make build` which uses additional build parameters for e.g. a smaller artifact size.

Of course, you can also use the [ZIP-archive](https://github.com/hauke96/tiny-http-proxy/archive/master.zip) if you don't have git installed.

Instead of `mkdir cache`, you have to make sure that the folder you'll configure later exists.

# CLI arguments

| Parameter | Description |
|:---|:---|
| `--help`, `-h` | Show this list of available parameters |
| `-config my-config.json` | Uses the file `my-config.json` as configuration file. Default value is `./tiny.json`.

# Configuration file
All the configuration is done in the `tiny.json` file. This is a simple JSON-file with some properties that should be set by you:

| Property | Type | Description |
|:---|:---|:---|
| `port` | `string` | The port this server is listening to. |
| `target` | `string` | The target host every request should be routed to. |
| `cache_folder` | `string` | The folder where the cache files are stored. This folder must exist and must be writable. |
| `debug_logging` | `bool` | When set to true, more detailed log output is printed. |
| `max_cache_item_size` | `int` | Maximum size in MB for the in-memory cache. Larger files are only cached on disk, smaller files are also cached directly within the memory. |

# Usage
If you normally go to `http://foo.com/bar?request=test` then now go to `http://localhost:8080/bar?request=test` (assumed there's a correct configuration).

# Exmaples
## With the given config
The current configuration caches requests to `https://imgs.xkcd.com`. So just start the proxy and go to e.g.:

[http://localhost:8080/comics/campaign_fundraising_emails.png](http://localhost:8080/comics/campaign_fundraising_emails.png)

## Caching google searches
Example: Create a proxy for google:

```json
{
    "port": "8080",
    "target": "http://www.google.com/",
    "cache_folder": "./cache/"
}
```
Then using the proxy with the URL `http://localhost:8080/search?source=hp&ei=QmBwWtTMHojOwAK2146oDQ&btnG=Suche&q=go+(language)` will open the result page of a google search for "go (language)". You may notice, that the site looks different then the original one. This happens because the proxy does not change links in the HTML (e.g. to `css` files).

The cache folder now contains files that are requested when opening the site (the HTML page, the favicon or other images):
```
5,4K 5b99ab35db77d3f6b8fada5270bc47b924ee8cca8b446d5d17cb6eed57bd372f
5,4K 802264eb0ff19278f578bfe80df00b9ed3b9ee67f670c2d6cea2d330cb7a49eb
152K 8988bca2a82bd9d3d52f03e0ecc7068db934d627f3a369f736e769360c968d93
   0 aaa75631890ab943c5ac2033591cb4287d1a6085604a74dc854a6117e6a0e104
 51K b9a229ca754de56be3c2743e5a51deac09e170c05388cc2c14b2e70608d9d4e4
 12K e5c601f8012efac42f37f43427272d2e9ec9756b2d401fab2a495dd3b96266bc
```
A great thing: Accessing another google search site, some images are not downloaded twice. Long live the cache!
