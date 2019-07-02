This binary hosts a HTTPS server and forwards unecrypted HTTP requests to configurable hosts, transparently fetching certificates [via Let's Encrypt](https://godoc.org/golang.org/x/crypto/acme/autocert).

## Why

You want to host lots of random services publicly on the internet, but don't want to generate certificates for all of them.
This takes care of that.

## Usage

For Ubuntu with systemd, you can:

* Check out this repo
* Run `./build.sh` (you'll need Go >1.12? and dependencies)
* Modify `https-forward.service` to point to the binary
* Run `./install.sh` to add to systemd

This service runs on port :80 (just redirects all requests to :443) and :443.

## Config

By default, this binary reads from `/etc/https-forward`, but can be configured via `--config path/to/config`.
Here's an example file:

```
host.example.com    localhost:8080

.yourdomain.com  # suffix all following
test                localhost:9000
basic-auth          localhost:9001    user:pass  # uses HTTP basic auth
user-only-auth      localhost:9002    user       # .. but doesn't care about password

.anotherdomain.com
test                blah.com          # any URL is valid, not just localhost
```

## Notes

This service will only ask Let's Encrypt for certificates when it can match a domain from the configuration exactly.
This prevents folks dialing your server and asking for random hostnames.

Having said that, a good way to use this is to set up wildcard DNS record.
For example, you could set up `*.yourdomain.com` to point your server's IP and then use the configuration file to add hosts.

