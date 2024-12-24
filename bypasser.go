package bypasser

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/imroc/req/v3"
)

type Bypasser struct {
	DevMode         bool
	browserMode     bool
	browserHeadless bool

	Transport http.RoundTripper
}

func NewBypasser(options ...func(*Bypasser)) (*Bypasser, error) {
	b := &Bypasser{
		DevMode:         false,
		browserMode:     false,
		browserHeadless: true,
	} //default

	for _, f := range options {
		f(b)
	}

	var err error
	if b.browserMode {
		b.Transport, err = NewBrowserRoundTripper(b.browserHeadless)
	} else {
		b.Transport, err = NewStandardRoundTripper(b.DevMode)
	}

	if err != nil {
		return nil, err
	}

	return b, err
}

// WithDevMode
func WithDevMode(dev bool) func(*Bypasser) {
	return func(b *Bypasser) {
		b.DevMode = dev
	}
}

// WithBrowserMode
func WithBrowserMode(mode bool) func(*Bypasser) {
	return func(b *Bypasser) {
		b.browserMode = mode
	}
}

// WithBrowserHeadless
func WithBrowserHeadless(headless bool) func(*Bypasser) {
	return func(b *Bypasser) {
		b.browserHeadless = headless
	}
}

type StandardRoundTripper struct {
	Client *req.Client
}

func NewStandardRoundTripper(devMode bool) (*StandardRoundTripper, error) {
	client := req.C().ImpersonateChrome()
	if devMode {
		client.DevMode()
	}

	return &StandardRoundTripper{
		Client: client,
	}, nil
}

func (b *StandardRoundTripper) RoundTrip(request *http.Request) (response *http.Response, err error) {
	for key, values := range request.Header {
		for _, value := range values {
			if key != "User-Agent" && key != "Cookie" {
				b.Client.SetCommonHeader(key, value)
			}
		}
	}
	b.Client.SetCommonCookies(request.Cookies()...)

	var resp *req.Response
	if request.Method == http.MethodHead {
		resp, err = b.Client.R().Head(request.URL.String())
	}

	if request.Method == http.MethodGet {
		resp, err = b.Client.R().Get(request.URL.String())
	}

	if request.Method == http.MethodPost {
		resp, err = b.Client.R().SetBody(request.Body).Post(request.URL.String())
	}

	if request.Method == http.MethodPut {
		resp, err = b.Client.R().SetBody(request.Body).Put(request.URL.String())
	}

	if request.Method == http.MethodDelete {
		resp, err = b.Client.R().Delete(request.URL.String())
	}

	response = resp.Response
	return response, err

}

type BrowserRoundTripper struct {
	headless bool
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
		headless: headless,
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

	html, err := page.HTML()
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:   200,
		Status:       "200 OK",
		Body:         io.NopCloser(strings.NewReader(html)),
		Header:       http.Header{"Content-Type": []string{"text/html"}},
		Uncompressed: true,
	}

	if networkResponse != nil {
		resp.StatusCode = networkResponse.Status
		resp.Status = networkResponse.StatusText
		resp.Header = http.Header{}

		for k, v := range networkResponse.Headers {
			resp.Header.Set(k, v.String())
		}
	}
	//resp.Header.Set("Content-Encoding", "*")
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
