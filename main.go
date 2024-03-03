package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
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
