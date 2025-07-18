package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	Proxy struct {
		port       int
		target     string
		hold       time.Duration
		targetUrl  *url.URL
		reqTimeMap sync.Map
		logDirName string
		logWriter  io.Writer
		noLog      []*regexp.Regexp

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
	logFn := fmt.Sprintf("log/%d.log", config.Port)
	fInfo, err := os.Stat(logFn)
	if err == nil && fInfo.Size() > 0 {
		logFnRename := fmt.Sprintf("log/%d-%s.log", config.Port, fInfo.ModTime().Format("20060102150405"))
		if err = os.Rename(logFn, logFnRename); err != nil {
			log.Println("Error renaming ", logFn, "to", logFnRename)
		}
	}
	logFile, err := os.Create(logFn)
	if err != nil {
		log.Fatalln("Error creating log file", logFn)
	}
	logWriter := io.MultiWriter(NewPrefixedWriter(os.Stdout, strconv.Itoa(config.Port)), logFile)
	noLog := make([]*regexp.Regexp, 0)
	for _, pattern := range config.NoLog {
		rx, err := regexp.Compile(pattern)
		if err != nil {
			log.Println(config.Port, ": Ignoring invalid noLog pattern", pattern, ", error:", err)
		} else {
			noLog = append(noLog, rx)
		}
	}
	result := &Proxy{
		port:       config.Port,
		target:     config.Target,
		hold:       config.Hold,
		reqTimeMap: sync.Map{},
		logDirName: logDirName,
		logWriter:  logWriter,
		targetUrl:  targetUrl,
		noLog:      noLog,
	}
	rp := &httputil.ReverseProxy{
		Director:       result.proxyDirector,
		ModifyResponse: result.proxyModifyResponse,
		ErrorHandler:   result.proxyErrorHandler,
	}
	result.reverseProxy = rp
	return result
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) Start() error {
	p.srv = http.Server{
		Addr:         ":" + strconv.Itoa(p.port),
		ReadTimeout:  0, // set to 0 in case client somehow took long time to upload the request
		WriteTimeout: 0, // this must be bigger than upstream resp time, otherwise client got empty resp, so we set to 0
		Handler:      p,
	}
	return p.srv.ListenAndServe()
}

func (p *Proxy) Shutdown(ctx context.Context) {
	p.srv.Shutdown(ctx)

}

func (p *Proxy) proxyDirector(req *http.Request) {
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

	noLog := false
	for _, rx := range p.noLog {
		if rx.Match([]byte(req.RequestURI)) {
			noLog = true
			break
		}
	}
	if !noLog {
		f, err := os.Create(fmt.Sprintf("%s/%d-req", p.logDirName, val))
		if err != nil {
			log.Println("error create req log:", err)
			return
		}
		defer f.Close()
		printReq(f, req)
		fmt.Fprintf(f, string(reqDump))
	}
	hAcceptEnc := req.Header.Get("Accept-Encoding")
	if strings.Contains(hAcceptEnc, "gzip") {
		req.Header.Del("Accept-Encoding")
	}
	if p.hold > 0 {
		time.Sleep(p.hold)
	}
}

func (p *Proxy) proxyModifyResponse(resp *http.Response) error {
	for _, rx := range p.noLog {
		if rx.Match([]byte(resp.Request.RequestURI)) {
			return nil
		}
	}

	var buf bytes.Buffer
	body := io.TeeReader(resp.Body, &buf)
	dumpResp := *resp
	dumpResp.Body = io.NopCloser(body)
	respDump, err := httputil.DumpResponse(&dumpResp, true)
	if err != nil {
		fmt.Println("error dumping resp", resp.Request.URL)
		return err
	}
	resp.Body = io.NopCloser(&buf)

	req := resp.Request
	key := req.RemoteAddr + " " + req.Method + " " + req.RequestURI
	val, found := p.reqTimeMap.Load(key)
	logResp(p.logWriter, resp, respDump, val.(int64))
	if !found {
		log.Println("request time not found")
		return nil
	}
	f, err := os.Create(fmt.Sprintf("%s/%d-resp", p.logDirName, val))
	if err != nil {
		log.Println("error create req log:", err)
		return nil
	}
	defer f.Close()
	printResp(f, resp)
	fmt.Fprint(f, string(respDump))
	return nil
}

func (p *Proxy) proxyErrorHandler(writer http.ResponseWriter, req *http.Request, err error) {
	format := "%s - - [%s] \"%s %s %s\"\n"
	reqDate := time.Now().Format("02/January/2006:15:04:05 -0700")
	f := p.logWriter
	fmt.Fprintf(f, format, req.RemoteAddr, reqDate, req.Method, req.RequestURI, req.Proto)
}

func printReq(f *os.File, r *http.Request) {
	_, err := fmt.Fprintln(f, r.URL.Scheme, "|", r.Host, "|", r.URL, "|", r.URL.Path, "|", r.URL.RawQuery, "|", r.Header)
	if err != nil {
		log.Println("error writing to file:", f, err)
	}
}

func printResp(f *os.File, r *http.Response) {
	_, err := fmt.Fprintln(f, r.Request.URL.Scheme, "|", r.Request.Host, "|", r.Request.URL, "|", r.Request.URL.Path, "|", r.Request.URL.RawQuery, "|", r.Header)
	if err != nil {
		log.Println("error writing to file:", f, err)
	}
}

func logResp(f io.Writer, resp *http.Response, data []byte, timestamp int64) {
	reqDate := time.Now().Format("02/January/2006:15:04:05 -0700")
	req := resp.Request
	reqHost := req.URL.Scheme + "://" + req.URL.Host
	format := "%s - - [%s] \"%s %s %s\" %d %d \"%s\" %d\n"
	//log.Println("requestURI:", req.RequestURI, req.URL)
	_, err := fmt.Fprintf(f, format, req.RemoteAddr, reqDate, req.Method, req.RequestURI, req.Proto, resp.StatusCode, len(data), reqHost, timestamp)
	if err != nil {
		log.Println("error logging:", err)
	}
}
