package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			log.Println(k, " ", v)
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
	header.Set("kubeflow-userid", "anonymous@kubeflow.org")
}

type proxy struct {
}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		req.URL.Scheme = "http"
		kfPipelineEndpoint := os.Getenv("KF_PIPELINES_ENDPOINT")
		req.URL.Host = kfPipelineEndpoint
	}
	log.Println("===============Request===================")
	log.Println("req.RemoteAddr ", req.RemoteAddr)
	log.Println("req.Method ", req.Method)
	log.Println("req.URL ", req.URL)
	log.Println("req.RequestURI ", req.RequestURI)
	log.Println("req.URL.Scheme ", req.URL.Scheme)
	log.Println("========================================")

	client := &http.Client{}

	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	delHopHeaders(req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}
	log.Println("===============req.Header====================")
	for k, vv := range req.Header {
		for _, v := range vv {
			log.Println(k, " ", v)
		}
	}
	log.Println("===========================================")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Fatal("ServeHTTP:", err)
	}
	defer resp.Body.Close()

	log.Println(req.RemoteAddr, " ", resp.Status)
    log.Println("===============resp.Header====================")
	for k, vv := range resp.Header {
		for _, v := range vv {
			log.Println(k, " ", v)
		}
	}
	log.Println("===========================================")
	delHopHeaders(resp.Header)

	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
}

func main() {
	var addr = flag.String("addr", "0.0.0.0:6996", "The addr of the application.")
	flag.Parse()

	handler := &proxy{}

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
