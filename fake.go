package main

import (
	"crypto/tls"
)

const (
	selfSignedCrt = `-----BEGIN CERTIFICATE-----
MIICHDCCAaKgAwIBAgIUOlvx6DkRzo0SZMwb7C1u+KwkNWcwCgYIKoZIzj0EAwIw
RTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGElu
dGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yNTA3MDMwNjE0MzdaFw0zNTA3MDEw
NjE0MzdaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYD
VQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwdjAQBgcqhkjOPQIBBgUrgQQA
IgNiAASwDxbgBeQyRKaCdkbB2cUPQrF6/FqWihp3SoiMrsPALToxmKdqa3odfgBE
8cJEIBX3+Wrjx6HyIlqM5Z7bwdri+r44G8js8EcHTSjjJHFKaC4H3ZQe86p+aXzK
tW1yYCujUzBRMB0GA1UdDgQWBBTQdD7vSZIO7zwLsIfRCBuUCX1+3DAfBgNVHSME
GDAWgBTQdD7vSZIO7zwLsIfRCBuUCX1+3DAPBgNVHRMBAf8EBTADAQH/MAoGCCqG
SM49BAMCA2gAMGUCMCsj8wTe1EfT8dFN0OyLWrdzMXrQea24bV4Fqq0Xqn13GYpF
VWCscJG4+MYXXnWGXAIxALxvAWOuGzmB3KSPqlKcpg27mZcuy1kpGXWFIZgkF62W
2vXKr9LseabQPYEZhaJE2Q==
-----END CERTIFICATE-----`
	selfSignedKey = `-----BEGIN EC PARAMETERS-----
BgUrgQQAIg==
-----END EC PARAMETERS-----
-----BEGIN EC PRIVATE KEY-----
MIGkAgEBBDDKHR5xQpvkC+jhKxiP3O6yUCEuGXIbcDiNB5dVTX9IpnyqPn6ZP0bG
UMJU9rN6it2gBwYFK4EEACKhZANiAASwDxbgBeQyRKaCdkbB2cUPQrF6/FqWihp3
SoiMrsPALToxmKdqa3odfgBE8cJEIBX3+Wrjx6HyIlqM5Z7bwdri+r44G8js8EcH
TSjjJHFKaC4H3ZQe86p+aXzKtW1yYCs=
-----END EC PRIVATE KEY-----`
)

var (
	selfSignedCert tls.Certificate
)

func init() {
	cert, err := tls.X509KeyPair([]byte(selfSignedCrt), []byte(selfSignedKey))
	if err != nil {
		panic("should never fail parsing self-signed certs?")
	}
	selfSignedCert = cert
}
