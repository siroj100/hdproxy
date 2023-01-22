package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

const (
	minTimeout = 15 * time.Second
)

var (
	err        error
	reqTimeMap sync.Map
	logDirName string
)

func main() {
	proxyConfs := InitConfig()
	fmt.Printf("proxyConfs: %+v\n", proxyConfs)
	logDirName = fmt.Sprintf("log-%d", port)
	if err = os.Mkdir(logDirName, 0700); err != nil && !os.IsExist(err) {
		log.Fatalln("error create log folder", err)
	}

	proxy := &httputil.ReverseProxy{
		Director:       proxyDirector,
		ModifyResponse: proxyModifyResponse,
	}
	http.Handle("/", &ProxyHandler{proxy})
	writeTimeout := 2 * hold
	if writeTimeout < minTimeout {
		writeTimeout = minTimeout
	}

	srv := http.Server{
		Addr:         ":" + strconv.Itoa(port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: writeTimeout,
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()
	fmt.Println("ready.")

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	fmt.Println("shutting down")
	os.Exit(0)
}

func proxyDirector(req *http.Request) {
	printReq(os.Stdout, req)
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		fmt.Println("error dumping req", req.URL)
		return
	}
	val := time.Now().UnixNano()
	reqTimeMap.Store(req, val)
	//req.Host = targetUrl.Host
	//req.URL.Scheme = targetUrl.Scheme
	//req.URL.Host = targetUrl.Host
	//req.URL.Path = targetUrl.Path + req.URL.Path
	f, err := os.Create(fmt.Sprintf("%s/%d-req", logDirName, val))
	if err != nil {
		log.Println("error create req log:", err)
		return
	}
	defer f.Close()
	printReq(f, req)
	fmt.Fprintf(f, string(reqDump))
	time.Sleep(hold)
}

func proxyModifyResponse(resp *http.Response) error {
	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Println("error dumping resp", resp.Request.URL)
		return err
	}
	val, found := reqTimeMap.Load(resp.Request)
	if !found {
		log.Println("request time not found")
		return nil
	}
	f, err := os.Create(fmt.Sprintf("%s/%d-resp", logDirName, val))
	if err != nil {
		log.Println("error create req log:", err)
		return nil
	}
	printResp(f, resp)
	fmt.Fprint(f, string(respDump))
	return nil
}

type ProxyHandler struct {
	p *httputil.ReverseProxy
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//printReq(r)
	ph.p.ServeHTTP(w, r)
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
