This binary hosts a HTTPS server and forwards unecrypted HTTP requests to other domains, serving and fetching certificates automatically via Let's Encrpyt.

## Why

You want to host lots of random services publicly on the internet, but don't want to generate certificates for all of them.
This takes care of that.

## Usage

For Ubuntu with systemd, you can:

* Check out this repo
* Run `./build.sh` (you might need Go dependencies)
* Modify `https-forward.service` to point to the binary
* Run `./install.sh` to add to systemd

This service runs on port :80 (just redirects all requests to :443) and :443.

## Config

By default, this binary reads from `/etc/https-forward`.
Here's an example config:

```
host.example.com    localhost:8080

.yourdomain.com  # suffix all following
test                localhost:9000
basic-auth          localhost:9001    user:pass
user-only-auth      localhost:9002    user

.anotherdomain.com
test                blah.com          # any URL is valid
```

## Notes

This service will only ask Let's Encrpyt for certificates when it can match a domain from the configuration exactly.
This prevents folks dialing your server and asking for arbitrary URLs.

Having said that, a good way to use this is to set up wildcard DNS record (e.g., set up `*.yourdomain.com` above to point your server's IP).

