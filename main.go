package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	proxies := make(map[int]*Proxy)
	proxyConfs := InitConfig()
	fmt.Printf("proxyConfs: %+v\n", proxyConfs)
	for port, conf := range proxyConfs {
		proxy := NewProxy(conf)
		go func() {
			if err := proxy.Start(); err != nil {
				log.Fatalln("Port", port, ":", err)
			}
		}()
		fmt.Println("Ready listening on port", port)
		proxies[port] = proxy

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
