package config

import (
	"flag"
	"log"
	"strings"
)

type Config struct {
	SelfPort    string
	PeerAddrs   []string
	PingFreq    int
	PingTimeout int
}

func Load() *Config {
	port := flag.String("port", "8081", "Port for the node to listen on")
	peers := flag.String("peers", "", "Comma-separated list of peer addresses (e.g., http://localhost:8002,http://localhost:8003)")
	pingFreq := flag.Int("pingfreq", 10, "Frequency of sending ping messages in seconds")
	pingTimeout := flag.Int("timeout", 15, "Timeout for ping responses in seconds")
	flag.Parse()

	if *peers == "" {
		log.Fatal("Peers address must be provided")
	}

	return &Config{
		SelfPort:    *port,
		PeerAddrs:   strings.Split(*peers, ","),
		PingFreq:    *pingFreq,
		PingTimeout: *pingTimeout,
	}
}
