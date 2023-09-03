package dpcore

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var onExitFlushLoop func()

// ReverseProxy is an HTTP Handler that takes an incoming request and
// sends it to another server, proxying the response back to the
// client, support http, also support https tunnel using http.hijacker
type ReverseProxy struct {
	// Set the timeout of the proxy server, default is 5 minutes
	Timeout time.Duration

	// Director must be a function which modifies
	// the request into a new request to be sent
	// using Transport. Its response is then copied
	// back to the original client unmodified.
	// Director must not access the provided Request
	// after returning.
	Director func(*http.Request)

	// The transport used to perform proxy requests.
	// default is http.DefaultTransport.
	Transport http.RoundTripper

	// FlushInterval specifies the flush interval
	// to flush to the client while copying the
	// response body. If zero, no periodic flushing is done.
	FlushInterval time.Duration

	// ErrorLog specifies an optional logger for errors
	// that occur when attempting to proxy the request.
	// If nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	// ModifyResponse is an optional function that
	// modifies the Response from the backend.
	// If it returns an error, the proxy returns a StatusBadGateway error.
	ModifyResponse func(*http.Response) error

	//Prepender is an optional prepend text for URL rewrite
	//
	Prepender string

	Verbal bool
}

type ResponseRewriteRuleSet struct {
	ProxyDomain  string
	OriginalHost string
	UseTLS       bool
	PathPrefix   string //Vdir prefix for root, / will be rewrite to this
}

type requestCanceler interface {
	CancelRequest(req *http.Request)
}

func NewDynamicProxyCore(target *url.URL, prepender string, ignoreTLSVerification bool) *ReverseProxy {
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}

		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "")
		}

	}

	//Hack the default transporter to handle more connections
	thisTransporter := http.DefaultTransport
	optimalConcurrentConnection := 32
	thisTransporter.(*http.Transport).MaxIdleConns = optimalConcurrentConnection * 2
	thisTransporter.(*http.Transport).MaxIdleConnsPerHost = optimalConcurrentConnection
	thisTransporter.(*http.Transport).IdleConnTimeout = 30 * time.Second
	thisTransporter.(*http.Transport).MaxConnsPerHost = optimalConcurrentConnection * 2
	thisTransporter.(*http.Transport).DisableCompression = true

	if ignoreTLSVerification {
		//Ignore TLS certificate validation error
		thisTransporter.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
	}

	return &ReverseProxy{
		Director:  director,
		Prepender: prepender,
		Verbal:    false,
		Transport: thisTransporter,
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}

	// Same as singleJoiningSlash, but uses EscapedPath to determine
	// whether a slash should be added
	apath := a.EscapedPath()
	bpath := b.EscapedPath()
	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")

	switch {
	case aslash && bslash:
		return a.Path + b.Path[1:], apath + bpath[1:]
	case !aslash && !bslash:
		return a.Path + "/" + b.Path, apath + "/" + bpath
	}

	return a.Path + b.Path, apath + bpath
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	//"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	//"Upgrade",
}

func (p *ReverseProxy) copyResponse(dst io.Writer, src io.Reader) {
	if p.FlushInterval != 0 {
		if wf, ok := dst.(writeFlusher); ok {
			mlw := &maxLatencyWriter{
				dst:     wf,
				latency: p.FlushInterval,
				done:    make(chan bool),
			}

			go mlw.flushLoop()
			defer mlw.stop()
			dst = mlw
		}
	}

	io.Copy(dst, src)
}

type writeFlusher interface {
	io.Writer
	http.Flusher
}

type maxLatencyWriter struct {
	dst     writeFlusher
	latency time.Duration
	mu      sync.Mutex
	done    chan bool
}

func (m *maxLatencyWriter) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dst.Write(b)
}

func (m *maxLatencyWriter) flushLoop() {
	t := time.NewTicker(m.latency)
	defer t.Stop()
	for {
		select {
		case <-m.done:
			if onExitFlushLoop != nil {
				onExitFlushLoop()
			}
			return
		case <-t.C:
			m.mu.Lock()
			m.dst.Flush()
			m.mu.Unlock()
		}
	}
}

func (m *maxLatencyWriter) stop() {
	m.done <- true
}

func (p *ReverseProxy) logf(format string, args ...interface{}) {
	if p.ErrorLog != nil {
		p.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func removeHeaders(header http.Header) {
	// Remove hop-by-hop headers listed in the "Connection" header.
	if c := header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				header.Del(f)
			}
		}
	}

	// Remove hop-by-hop headers
	for _, h := range hopHeaders {
		if header.Get(h) != "" {
			header.Del(h)
		}
	}

	if header.Get("A-Upgrade") != "" {
		header.Set("Upgrade", header.Get("A-Upgrade"))
		header.Del("A-Upgrade")
	}
}

func addXForwardedForHeader(req *http.Request) {
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		req.Header.Set("X-Forwarded-For", clientIP)
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}

	}
}

func (p *ReverseProxy) ProxyHTTP(rw http.ResponseWriter, req *http.Request, rrr *ResponseRewriteRuleSet) error {
	transport := p.Transport

	outreq := new(http.Request)
	// Shallow copies of maps, like header
	*outreq = *req

	if cn, ok := rw.(http.CloseNotifier); ok {
		if requestCanceler, ok := transport.(requestCanceler); ok {
			// After the Handler has returned, there is no guarantee
			// that the channel receives a value, so to make sure
			reqDone := make(chan struct{})
			defer close(reqDone)
			clientGone := cn.CloseNotify()

			go func() {
				select {
				case <-clientGone:
					requestCanceler.CancelRequest(outreq)
				case <-reqDone:
				}
			}()
		}
	}

	p.Director(outreq)
	outreq.Close = false

	if !rrr.UseTLS {
		//This seems to be routing to external sites
		//Do not keep the original host
		outreq.Host = rrr.OriginalHost
	}

	// We may modify the header (shallow copied above), so we only copy it.
	outreq.Header = make(http.Header)
	copyHeader(outreq.Header, req.Header)

	// Remove hop-by-hop headers listed in the "Connection" header, Remove hop-by-hop headers.
	removeHeaders(outreq.Header)

	// Add X-Forwarded-For Header.
	addXForwardedForHeader(outreq)

	res, err := transport.RoundTrip(outreq)
	if err != nil {
		if p.Verbal {
			p.logf("http: proxy error: %v", err)
		}

		//rw.WriteHeader(http.StatusBadGateway)
		return err
	}

	// Remove hop-by-hop headers listed in the "Connection" header of the response, Remove hop-by-hop headers.
	removeHeaders(res.Header)

	if p.ModifyResponse != nil {
		if err := p.ModifyResponse(res); err != nil {
			if p.Verbal {
				p.logf("http: proxy error: %v", err)
			}

			//rw.WriteHeader(http.StatusBadGateway)
			return err
		}
	}

	//Custom header rewriter functions
	if res.Header.Get("Location") != "" {
		locationRewrite := res.Header.Get("Location")
		originLocation := res.Header.Get("Location")
		res.Header.Set("zr-origin-location", originLocation)

		if strings.HasPrefix(originLocation, "http://") || strings.HasPrefix(originLocation, "https://") {
			//Full path
			//Replace the forwarded target with expected Host
			lr, err := replaceLocationHost(locationRewrite, rrr, req.TLS != nil)
			if err == nil {
				locationRewrite = lr
			}
		} else if strings.HasPrefix(originLocation, "/") && rrr.PathPrefix != "" {
			//Back to the root of this proxy object
			//fmt.Println(rrr.ProxyDomain, rrr.OriginalHost)
			locationRewrite = strings.TrimSuffix(rrr.PathPrefix, "/") + originLocation
		} else {
			//Relative path. Do not modifiy location header

		}

		//Custom redirection to this rproxy relative path
		res.Header.Set("Location", locationRewrite)
	}

	// Copy header from response to client.
	copyHeader(rw.Header(), res.Header)

	// The "Trailer" header isn't included in the Transport's response, Build it up from Trailer.
	if len(res.Trailer) > 0 {
		trailerKeys := make([]string, 0, len(res.Trailer))
		for k := range res.Trailer {
			trailerKeys = append(trailerKeys, k)
		}
		rw.Header().Add("Trailer", strings.Join(trailerKeys, ", "))
	}

	rw.WriteHeader(res.StatusCode)
	if len(res.Trailer) > 0 {
		// Force chunking if we saw a response trailer.
		// This prevents net/http from calculating the length for short
		// bodies and adding a Content-Length.
		if fl, ok := rw.(http.Flusher); ok {
			fl.Flush()
		}
	}

	p.copyResponse(rw, res.Body)
	// close now, instead of defer, to populate res.Trailer
	res.Body.Close()
	copyHeader(rw.Header(), res.Trailer)

	return nil
}

func (p *ReverseProxy) ProxyHTTPS(rw http.ResponseWriter, req *http.Request) error {
	hij, ok := rw.(http.Hijacker)
	if !ok {
		p.logf("http server does not support hijacker")
		return errors.New("http server does not support hijacker")
	}

	clientConn, _, err := hij.Hijack()
	if err != nil {
		if p.Verbal {
			p.logf("http: proxy error: %v", err)
		}
		return err
	}

	proxyConn, err := net.Dial("tcp", req.URL.Host)
	if err != nil {
		if p.Verbal {
			p.logf("http: proxy error: %v", err)
		}

		return err
	}

	// The returned net.Conn may have read or write deadlines
	// already set, depending on the configuration of the
	// Server, to set or clear those deadlines as needed
	// we set timeout to 5 minutes
	deadline := time.Now()
	if p.Timeout == 0 {
		deadline = deadline.Add(time.Minute * 5)
	} else {
		deadline = deadline.Add(p.Timeout)
	}

	err = clientConn.SetDeadline(deadline)
	if err != nil {
		if p.Verbal {
			p.logf("http: proxy error: %v", err)
		}
		return err
	}

	err = proxyConn.SetDeadline(deadline)
	if err != nil {
		if p.Verbal {
			p.logf("http: proxy error: %v", err)
		}

		return err
	}

	_, err = clientConn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
	if err != nil {
		if p.Verbal {
			p.logf("http: proxy error: %v", err)
		}

		return err
	}

	go func() {
		io.Copy(clientConn, proxyConn)
		clientConn.Close()
		proxyConn.Close()
	}()

	io.Copy(proxyConn, clientConn)
	proxyConn.Close()
	clientConn.Close()

	return nil
}

func (p *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request, rrr *ResponseRewriteRuleSet) error {
	if req.Method == "CONNECT" {
		err := p.ProxyHTTPS(rw, req)
		return err
	} else {
		err := p.ProxyHTTP(rw, req, rrr)
		return err
	}
}
