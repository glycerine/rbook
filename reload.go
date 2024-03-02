package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

// portions of this code reused under this license
/*
MIT License

Copyright (c) 2016 Mark Vincze

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

var _ bytes.Buffer

var (
	hub *Hub
	// The port on which we are hosting the reload server has to be hardcoded on the client-side too.
	//reloadAddress    = ":12450"
	//reloadAddressTLS = ":12451"
)

const (
	certContent = `-----BEGIN CERTIFICATE-----
MIICkzCCAfwCCQCbmnQ2PFatzzANBgkqhkiG9w0BAQsFADCBjTELMAkGA1UEBhMC
TkwxEzARBgNVBAgMClNvbWUtU3RhdGUxEjAQBgNVBAcMCUFtc3RlcmRhbTEPMA0G
A1UECgwGVHJhdml4MQ0wCwYDVQQLDARDb3JlMRIwEAYDVQQDDAlsb2NhbGhvc3Qx
ITAfBgkqhkiG9w0BCQEWEm12aW5jemVAdHJhdml4LmNvbTAeFw0xNjEwMTUxNzEy
NTVaFw0xOTA4MDUxNzEyNTVaMIGNMQswCQYDVQQGEwJOTDETMBEGA1UECAwKU29t
ZS1TdGF0ZTESMBAGA1UEBwwJQW1zdGVyZGFtMQ8wDQYDVQQKDAZUcmF2aXgxDTAL
BgNVBAsMBENvcmUxEjAQBgNVBAMMCWxvY2FsaG9zdDEhMB8GCSqGSIb3DQEJARYS
bXZpbmN6ZUB0cmF2aXguY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCf
OSB7LkkaRd6WXTplUHfD2k+EHoVi9flKcmbUlye9zHFzWVtCUQjhFjiZL1rNRQGn
9VMUqzpc55RyzTEy2KpyZ+7INR1ZAuqqXMxpNDzXeq+UQuAFnJrHnbwtiSYPiJ45
5EvysllYb5j6ihXEVZt+6QdMINFB+Gz0Xfrhug0+0QIDAQABMA0GCSqGSIb3DQEB
CwUAA4GBADrH8ibFye3iXHR6RkwVNBgeKyvL0kxs4C8785uYqjRJWVjAg2xJQyyZ
R3IHuvKqkmjs5i5d5CT9QT4t8Mlorg1XSnRz/HLf5zrRJlVzqrpd9N2+859TmTVD
9A91NtEwCNgBSGDGSCndjQ/dkPhbJFs28/ICujLySxbYswOGHGbK
-----END CERTIFICATE-----
`
	keyContent = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCfOSB7LkkaRd6WXTplUHfD2k+EHoVi9flKcmbUlye9zHFzWVtC
UQjhFjiZL1rNRQGn9VMUqzpc55RyzTEy2KpyZ+7INR1ZAuqqXMxpNDzXeq+UQuAF
nJrHnbwtiSYPiJ455EvysllYb5j6ihXEVZt+6QdMINFB+Gz0Xfrhug0+0QIDAQAB
AoGAIo9SxonwYhyCSN7peu4xYLh1A/df+m/rcUZNnZ1FigPjKCdgEI/oPnsFQ/Ks
Ydu1lVBBfT4BSAMYDKcPI7s1m5Hf++2TAWXuE/GiMmfmQq8QHVwdRERIzGo7BSIW
alA5tC4+dIe5gUKjR38MpG9VCEa3FBkNxlRQ2U1tIAoM9/ECQQDLWvbShPYpfKCM
8WlAGeWwgHJrjdmatMLsJepxFjGShxK1uhLy6mIMaVVCV0dFPk2Y81ACAirmev99
bqMd3sbtAkEAyHFgTZzQUrezQQhnfFcEDOaUrCwRBVERHFou6wHEwTLObJeedAuo
emRRpQkOp+wJq8y9eOI2pv0jpSI8pTKW9QJAdOuzOG1sX4Qhh4gSHOIG90mTABYK
BHJkFITkW+sHy5jQAB6hYHu0rjAt7jviZYSh9wwGd3Epm2Ui2sqvDLCXLQJBAKAk
NNTNXIM50TU8CbIFs267Kj0EV/Tvd8Q3KRUJLLFObi3EVQxR5CEk1TYNrm/q3S8t
PJO/5/oydLASUnGJoaECQGyPpJ6lVJb10yJKjcGtouwa+HFRJh9BxIQUHZRTbmHX
k7iRrF0Vcllo8k/Mos5PVPP0WIyS1l0lh4GZ+w8gA80=
-----END RSA PRIVATE KEY-----
`
)

func createCertFiles() (cert string, key string) {
	tempFolder, _ := ioutil.TempDir("", "reload")

	cert = tempFolder + "/reload-cert.pem"
	key = tempFolder + "/reload-key.pem"

	ioutil.WriteFile(cert, []byte(certContent), 0644)
	ioutil.WriteFile(key, []byte(keyContent), 0644)

	return cert, key
}

func (cfg *RbookConfig) startReloadServer(book *HashRBook) {
	hub = newHub(book)
	go hub.runRestarter() // never returns, recovers from all panics on its goroutine.
	http.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
		vvlog("serveWs has returned: request was r = '%#v'", r)
	})

	go startServer(cfg)
	go startServerTLS(cfg)
	//vv("Reload server listening at '%v'", reloadAddress)
}

func startServer(cfg *RbookConfig) {
	err := http.ListenAndServe(fmt.Sprintf(":%v", cfg.WsPort), nil)
	vvlog("startServer ListenAndServe has exited with err= '%v'", err)
	panicOn(err)
}

func startServerTLS(cfg *RbookConfig) {
	cert, key := createCertFiles()
	err := http.ListenAndServeTLS(fmt.Sprintf(":%v", cfg.WssPort), cert, key, nil)
	vvlog("startServerTLS ListenAndServeTLS has exited with err= '%v'", err)
	panicOn(err)
}
