Provides a forwarding HTTPS server which transparently fetches and caches certificates [via Let's Encrypt](https://godoc.org/golang.org/x/crypto/acme/autocert).
This must run on 443 and 80 (just forwards to https://) and can't coexist with any other web server on your machine.

## Why

This is so you can host random and long-lived services publicly on the internet—perfect for _other_ services which don't care about certificates or HTTPS at all, and might be provided by Node or Go on a random high port (e.g., some dumb service running on `localhost:8080`).

**Note!** This doesn't magic up domain names.
You would use this service only if you're able to point DNS records to the IP address of a machine you're running this on, and that the machine is able to handle incoming requests on port 443 and 80 (e.g., on a home network, you'd have to set up port forwarding on your router).

## Install

You should probably install this via [Snap](https://snapcraft.io/https-forward) if you're using Ubuntu or something like it.

Otherwise, you can build the Go binary and see `--help` for flags.
You _should_ restrict the binary's permissions or run it as `nobody` with a `setcap` configuration that lets it listen on low ports.

## Configuration

If you're using Snap, the configuration file is at `/var/snap/https-forward/common/config` (which is empty after install).
Otherwise, the default configuration is read at `/etc/https-forward`.

Either way, it should be authored like this:

    # hostname            forward-to          optional-basic-auth
    host.example.com      localhost:8080
    blah.example.com      192.168.86.24:7999  user:pass
    user-only.example.com localhost:9002      user       # accepts any password
   
    # ... specify host with '.' to suffix all following
    .example.com
    test                  localhost:9000
    under-example         any-hostname-here.com:9000

(example.com used above purely as an _example_.
You'd replace it with a domain name you controlled, preferably with a [wildcard DNS](https://en.wikipedia.org/wiki/Wildcard_DNS_record) record.)

Restart or send `SIGHUP` to the binary to reread the config file.

## Notes

If incoming HTTPS requests take a long time and then fail, Let's Encrypt might have throttled you.
Unfortunatley, the `autocert` client in Go isn't very verbose about this.
This happens on a per-domain basis (rather than say, from your client IP), so just try a new domain (even a subdomain).
