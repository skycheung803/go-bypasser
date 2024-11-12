package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	"github.com/skycheung803/go-bypasser"
)

func main() {
	log.Println("starting~~~~")
	//bypasserTest()
	//log.Println("bypasser finish~~~~")
	//bypasserTest("browser")
	//log.Println("bypasser finish~~~~")
	//standard()
	//log.Println("standard finish~~~~")
	//browser()
	gocolly()
	log.Println("browser finish~~~~")
}

func bypasserTest() {
	bypass, err := bypasser.NewBypasser()
	//bypass, err := bypasser.NewBypasser(bypasser.WithBrowserMode(true), bypasser.WithBrowserHeadless(false))
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

func standard() {
	tr, err := bypasser.NewStandardRoundTripper(tlsclient.WithClientProfile(profiles.Chrome_124))
	if err != nil {
		log.Fatal(err)
	}

	//tr := bypasser.StandardRoundTripper{}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	req, err := http.NewRequest("GET", "https://httpbin.org/anything", nil)
	if err != nil {
		log.Fatal(err)
	}

	//req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	//fmt.Printf("%v \n", resp.Header)
	if err != nil {
		log.Fatal(err)
	}
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bodyText))
}

func browser() {

	tr, err := bypasser.NewBrowserRoundTripper(false)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("GET", "https://httpbin.org/anything", nil)
	if err != nil {
		log.Fatal(err)
	}

	//req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	//req.Header.Set("my-key", "my-value33333") //Error, has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header is present on the requested resource.
	//tr := &http.Transport{}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: tr,
	}

	resp, err := client.Do(req)
	//fmt.Printf("%v \n", resp.Header)
	if err != nil {
		log.Fatal(err)
	}
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bodyText))

	//runtime.Goexit()
}

func gocolly() {
	c := colly.NewCollector()
	extensions.RandomUserAgent(c)

	//bypass, err := bypasser.NewBypasser()
	//bypass, err := bypasser.NewBypasser(bypasser.WithBrowserMode(true), bypasser.WithBrowserHeadless(false))
	bypass, err := bypasser.NewBypasser(bypasser.WithBrowserMode(true))
	if err != nil {
		log.Fatal(err)
	}

	c.WithTransport(bypass.Transport)

	c.OnResponse(func(r *colly.Response) {
		fmt.Println(string(r.Body))
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.Visit("https://httpbin.org/anything")
}
