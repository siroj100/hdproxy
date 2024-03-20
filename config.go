package main

import (
	"flag"
	"io"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type (
	ProxyConfig struct {
		Port   int
		Target string
		Hold   time.Duration
		NoLog  []string
	}
)

func InitConfig() []ProxyConfig {
	var (
		fname  string
		port   int
		target string
		hold   time.Duration
	)

	flag.StringVar(&fname, "config", "", "config file to read")
	flag.IntVar(&port, "port", 0, "local port to listen to")
	flag.StringVar(&target, "target", "", "target URL to proxy to")
	flag.DurationVar(&hold, "hold", 0, "how long to hold the request")
	flag.Parse()
	target = strings.TrimSpace(target)
	if port != 0 && len(target) > 0 {
		_, err := url.Parse(target)
		if err != nil {
			log.Fatalln("invalid target url")
		}
		result := make([]ProxyConfig, 1)
		result[0] = ProxyConfig{
			Port:   port,
			Target: target,
			Hold:   hold,
		}
		return result
	}
	fname = strings.TrimSpace(fname)
	if len(fname) < 1 {
		fname = "hdproxy.toml"
	}
	fstream, err := os.Open(fname)
	if err != nil {
		log.Fatalln("no port or target provided, and failed to read config file,", err)
	}
	result := ReadConfig(fstream)
	//fmt.Printf("%+v\n", result)
	return result
}

func ReadConfig(stream io.Reader) []ProxyConfig {
	var configs map[int]ProxyConfig
	viper.SetConfigType("toml")
	err := viper.ReadConfig(stream)
	if err != nil {
		log.Fatalln("can't read config", err)
	}
	err = viper.Unmarshal(&configs)
	if err != nil {
		log.Fatalln("can't parse config", err)
	}
	result := make([]ProxyConfig, len(configs))
	i := 0
	for k := range configs {
		cfg := configs[k]
		cfg.Port = k
		configs[k] = cfg
		result[i] = cfg
		i += 1
		//fmt.Printf("%i: %+v\n", k, cfg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Port < result[j].Port
	})
	return result
}
