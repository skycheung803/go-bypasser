package bypasser

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

type Bypasser struct {
	BrowserMode     bool
	BrowserHeadless bool
	Transport       http.RoundTripper
}

func NewBypasser(options ...func(*Bypasser)) (*Bypasser, error) {
	b := &Bypasser{
		BrowserMode:     false,
		BrowserHeadless: true,
	} //default

	for _, f := range options {
		f(b)
	}

	var err error
	if b.BrowserMode {
		b.Transport, err = NewBrowserRoundTripper(b.BrowserHeadless)
	} else {
		b.Transport, err = NewStandardRoundTripper(tlsclient.WithClientProfile(profiles.Chrome_124))
	}

	if err != nil {
		return nil, err
	}

	return b, err
}

// WithBrowserMode
func WithBrowserMode(mode bool) func(*Bypasser) {
	return func(b *Bypasser) {
		b.BrowserMode = mode
	}
}

// WithBrowserHeadless
func WithBrowserHeadless(headless bool) func(*Bypasser) {
	return func(b *Bypasser) {
		b.BrowserHeadless = headless
	}
}

type StandardRoundTripper struct {
	Client tlsclient.HttpClient
}

func NewStandardRoundTripper(httpClientOption ...tlsclient.HttpClientOption) (*StandardRoundTripper, error) {
	c, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), httpClientOption...)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}

	return &StandardRoundTripper{
		Client: c,
	}, nil
}

func (b *StandardRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fReq, err := fhttp.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		return nil, err
	}

	//log.Println(fReq.Header.Get("User-Agent"))

	fReq.Header = fhttp.Header(req.Header)
	fReq.Trailer = fhttp.Header(req.Trailer)
	fReq.Form = req.Form
	fReq.MultipartForm = req.MultipartForm
	fReq.PostForm = req.PostForm

	//log.Println(req.Header.Get("User-Agent"))
	//log.Println(fReq.Header.Get("User-Agent"))

	fResp, err := b.Client.Do(fReq)
	if err != nil {
		return nil, fmt.Errorf("error fetching response: %w", err)
	}

	return &http.Response{
		Status:           fResp.Status,
		StatusCode:       fResp.StatusCode,
		Proto:            fResp.Proto,
		ProtoMajor:       fResp.ProtoMajor,
		ProtoMinor:       fResp.ProtoMinor,
		Header:           http.Header(fResp.Header),
		Body:             fResp.Body,
		ContentLength:    fResp.ContentLength,
		TransferEncoding: fResp.TransferEncoding,
		Close:            fResp.Close,
		Uncompressed:     fResp.Uncompressed,
		Trailer:          http.Header(fResp.Trailer),
		Request:          req,
		TLS:              nil,
	}, nil
}

type BrowserRoundTripper struct {
	Headless bool
	Browser  *rod.Browser
}

func newBrowser(headless bool) (*rod.Browser, error) {
	path, _ := launcher.LookPath()
	serviceURL, err := launcher.New().Bin(path).Headless(headless).Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(serviceURL).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	return browser, nil
}

func NewBrowserRoundTripper(headless bool) (*BrowserRoundTripper, error) {
	browser, err := newBrowser(headless)
	if err != nil {
		return nil, err
	}

	return &BrowserRoundTripper{
		Headless: headless,
		Browser:  browser,
	}, nil
}

func (b *BrowserRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	default:
	}

	page := stealth.MustPage(b.Browser)
	defer page.Close()

	var once sync.Once
	var networkResponse *proto.NetworkResponse
	go page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if e.Type != proto.NetworkResourceTypeDocument {
			return
		}
		once.Do(func() {
			networkResponse = e.Response
		})
	})()

	page = page.Context(req.Context())

	for h := range req.Header {
		if h == "Cookie" {
			continue
		}
		if h == "User-Agent" && strings.HasPrefix(req.UserAgent(), "bypasser") {
			continue
		}
		page.MustSetExtraHeaders(h, req.Header.Get(h))
	}

	page.SetCookies(parseCookies(req))

	if err := page.Navigate(req.URL.String()); err != nil {
		return nil, err
	}

	timeout := page.Timeout(10 * time.Second)
	timeout.WaitLoad()
	timeout.WaitDOMStable(300*time.Millisecond, 0)
	timeout.WaitRequestIdle(time.Second, nil, nil, nil)
	//page.MustWaitStable()

	html, err := page.HTML()
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(html)),
		Header:     http.Header{"Content-Type": []string{"text/html"}},
	}

	if networkResponse != nil {
		resp.StatusCode = networkResponse.Status
		resp.Status = networkResponse.StatusText
		resp.Header = http.Header{}

		for k, v := range networkResponse.Headers {
			resp.Header.Set(k, v.String())
		}
	}

	return resp, err
}

func parseCookies(r *http.Request) []*proto.NetworkCookieParam {
	rawCookie := r.Header.Get("Cookie")
	if rawCookie == "" {
		return nil
	}

	header := http.Header{}
	header.Add("Cookie", rawCookie)
	request := http.Request{Header: header}

	domainSegs := strings.Split(r.URL.Hostname(), ".")
	if len(domainSegs) < 2 {
		return nil
	}

	domain := "." + strings.Join(domainSegs[len(domainSegs)-2:], ".")

	var cookies []*proto.NetworkCookieParam
	for _, cookie := range request.Cookies() {
		cookies = append(cookies, &proto.NetworkCookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   domain,
			Path:     "/",
			Secure:   false,
			HTTPOnly: false,
			SameSite: "Lax",
			Expires:  -1,
			URL:      r.URL.String(),
		})
	}

	return cookies
}
