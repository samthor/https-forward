Provides a forwarding HTTPS server which transparently fetches and caches certificates [via Let's Encrypt](https://godoc.org/golang.org/x/crypto/acme/autocert).
This must run on 443 and 80 (just forwards to https://) and can't coexist with any other web server on your machine.

## Why

This is so you can host random and long-lived services publicly on the internet—perfect services which don't care about certificates or HTTPS at all.
A good way to use this is to set up a wildcard record like `*.example.com` to point to your server (but don't worry, this service won't respond unless you explicitly define a host in your configuration file).

## Install

You should probably install this via [Snap](https://snapcraft.io/https-forward) if you're using Ubuntu or something like it.

Otherwise, you can build the Go binary and it will default to reading its config from `/etc/https-forward` (but see `--help` for flags).
You should restrict the binary's permissions or run it as `nobody` with a `setcap` configuration that lets it listen on low ports.

## Configuration

Configure this via `/var/snap/https-forward/common/config`, which is empty after install. It should be authored like this:

    # hostname            forward-to          optional-basic-auth
    host.example.com      localhost:8080
    blah.example.com      192.168.86.24:7999  user:pass
    user-only.example.com localhost:9002      user       # accepts any password
   
    # ... specify host with '.' to suffix all following
    .example.com
    test                  localhost:9000
    under-example         any-hostname-here.com:9000

Restart or send `SIGHUP` to the binary to reread the config file.

## Notes

If incoming HTTPS requests take a long time and then fail, Let's Encrypt might have throttled you.
Unfortunatley, the `autocert` client in Go isn't very verbose about this.
This happens on a per-domain basis (rather than say, from your client IP), so just try a new domain (even a subdomain).
