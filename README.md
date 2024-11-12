# go-bypasser
A Go's http.RoundTripper implementation that provides a wrapper for tls-client and leverages uTLS to spoof TLS fingerprints (JA3, JA4, HTTP/2 Akamai, etc) of mainstream browsers for use in different HTTP client libraries (like resty) to bypass Cloudflare or other firewalls.

## Features

- Customized TLS Cipher Suites.
- Customized TLS Extensions.
- Built-in fingerprint profiles of mainstream browsers.
- Implements Go's http.RoundTripper so can be used in different 3rd-party HTTP client libraries.
- Browsers Mode (The Browser Mode controls the interaction with a headless Chromium browser. Enabling the browser mode allows to download a Chromium browser once and use it to render JavaScript-heavy pages.)

## Install

```bash
go get -u github.com/skycheung803/go-bypasser
```

## Usage

```go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/skycheung803/go-bypasser"
)

func main() {
	// Create a Bypasser that implements the http.RoundTripper interface
	//bypass, err := bypasser.NewBypasser()
	//bypass, err := bypasser.NewBypasser(bypasser.WithBrowserMode(true),bypasser.WithBrowserHeadless(false))
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: bypass.Transport,
	}

	req, err := http.NewRequest("GET", "https://httpbin.org/anything", nil)
	if err != nil {
		log.Fatal(err)
	}
	//req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bodyText))
}
```

## credits  
__go-bypasser__ would not have been possible without some of [these amazing projects](./go.mod): [tls-client](github.com/bogdanfinn/tls-client), [go-rod](https://github.com/go-rod/rod), [fhttp](https://github.com/useflyent/fhttp)