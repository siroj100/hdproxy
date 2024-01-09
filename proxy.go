package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	minTimeout = 15 * time.Second
)

type (
	Proxy struct {
		port       int
		target     string
		hold       time.Duration
		targetUrl  *url.URL
		reqTimeMap sync.Map
		logDirName string

		srv          http.Server
		reverseProxy *httputil.ReverseProxy
	}
)

func NewProxy(config ProxyConfig) *Proxy {
	targetUrl, err := url.Parse(config.Target)
	if err != nil {
		log.Fatalln("invalid target url")
	}
	logDirName := fmt.Sprintf("log/%d", config.Port)
	if err = os.MkdirAll(logDirName, 0700); err != nil && !os.IsExist(err) {
		log.Fatalln("error create log folder", err)
	}
	result := &Proxy{
		port:       config.Port,
		target:     config.Target,
		hold:       config.Hold,
		reqTimeMap: sync.Map{},
		logDirName: logDirName,
		targetUrl:  targetUrl,
	}
	rp := &httputil.ReverseProxy{
		Director:       result.proxyDirector,
		ModifyResponse: result.proxyModifyResponse,
	}
	result.reverseProxy = rp
	return result
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) Start() error {
	writeTimeout := 2 * p.hold
	if writeTimeout < minTimeout {
		writeTimeout = minTimeout
	}
	p.srv = http.Server{
		Addr:         ":" + strconv.Itoa(p.port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: writeTimeout,
		Handler:      p,
	}
	return p.srv.ListenAndServe()
}

func (p *Proxy) Shutdown(ctx context.Context) {
	p.srv.Shutdown(ctx)

}

func (p *Proxy) proxyDirector(req *http.Request) {
	printReq(os.Stdout, req)
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		fmt.Println("error dumping req", req.URL)
		return
	}
	val := time.Now().UnixNano()
	key := req.RemoteAddr + " " + req.Method + " " + req.RequestURI
	p.reqTimeMap.Store(key, val)
	req.Host = p.targetUrl.Host
	req.URL.Scheme = p.targetUrl.Scheme
	req.URL.Host = p.targetUrl.Host
	req.URL.Path = p.targetUrl.Path + req.URL.Path
	f, err := os.Create(fmt.Sprintf("%s/%d-req", p.logDirName, val))
	if err != nil {
		log.Println("error create req log:", err)
		return
	}
	defer f.Close()
	printReq(f, req)
	fmt.Fprintf(f, string(reqDump))
	time.Sleep(p.hold)
}

func (p *Proxy) proxyModifyResponse(resp *http.Response) error {
	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Println("error dumping resp", resp.Request.URL)
		return err
	}
	req := resp.Request
	key := req.RemoteAddr + " " + req.Method + " " + req.RequestURI
	val, found := p.reqTimeMap.Load(key)
	if !found {
		log.Println("request time not found")
		return nil
	}
	f, err := os.Create(fmt.Sprintf("%s/%d-resp", p.logDirName, val))
	if err != nil {
		log.Println("error create req log:", err)
		return nil
	}
	printResp(f, resp)
	fmt.Fprint(f, string(respDump))
	return nil
}
