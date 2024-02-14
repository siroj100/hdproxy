package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	proxies := make(map[int]*Proxy)
	proxyConfs := InitConfig()
	//fmt.Printf("proxyConfs: %+v\n", proxyConfs)
	for _, conf := range proxyConfs {
		proxy := NewProxy(conf)
		go func() {
			if err := proxy.Start(); err != nil {
				log.Fatalln("Port", conf.Port, ":", err)
			}
		}()
		fmt.Println(conf.Port, "->", conf.Target, "hold:", conf.Hold)
		proxies[conf.Port] = proxy

	}

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	fmt.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, proxy := range proxies {
		proxy.Shutdown(ctx)
	}
	os.Exit(0)
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

func logResp(f io.Writer, resp *http.Response, data []byte) {
	reqDate := time.Now().Format("02/January/2006:15:04:05 -0700")
	req := resp.Request
	remoteAddr := strings.Split(req.RemoteAddr, ":")
	reqHost := req.URL.Scheme + "://" + req.URL.Host
	format := "%s - - [%s] \"%s %s %s\" %d %d \"%s\" \"%s\"\n"
	//log.Println("requestURI:", req.RequestURI, req.URL)
	_, err := fmt.Fprintf(f, format, remoteAddr[0], reqDate, req.Method, req.RequestURI, req.Proto, resp.StatusCode, len(data), reqHost, req.UserAgent())
	if err != nil {
		log.Println("error logging:", err)
	}
}
