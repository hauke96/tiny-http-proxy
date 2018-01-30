# tiny-http-proxy
Maybe the tiniest HTTP proxy that also has a cache.

The tiny-http-proxy acts as a reverse proxy for one server of your choice illustrated by this picture:

```
you ---> network <---> proxy ---> cache
            |
            V
       other site
```
Where "network" may be anything (LAN/WAN/...).

# Installation
Just clone this repo and run it:

```
git clone https://github.com/hauke96/tiny-http-proxy.git
cd tiny-http-proxy
go run *.go
```
Of course you can also use the [ZIP-archive](https://github.com/hauke96/tiny-http-proxy/archive/master.zip) if you don't have git installed.

# Configuration
All the configuration is done in the `tiny.json` file. This is a simple JSON-file with some properties that should be set by you:

| Property | Type | Description |
|:---|:---|:---|
| `port` | `string` | The port this server is listening to. |
| `target` | `string` | The target host every request should be routed to. |
| `cache_folder` | `string` | The folder where the cache files are stored. This folder must exist and must be writable. |

# Usage
If you normally go to `http://foo.com/bar?request=test` then now go to `http://localhost:8080/bar?request=test` (assumed there's a correct configuration).

# Example
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
