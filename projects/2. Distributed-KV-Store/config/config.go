package config

import (
	"flag"
	"log"
	"strconv"
	"strings"
)

// Config represents the configuration for the distributed key-value store.
// It includes the port on which the node listens, a list of peer nodes,
// the frequency of pinging peers, and a timeout for operations.
type Config struct {
	Port          string
	Peers         []string
	PingFrequency int
	Timeout       int
}

func Load() *Config {
	port := flag.String("port", "", "Port on which the node listens")
	peers := flag.String("peers", "", "Comma-separated list of peer nodes (example: http://localhost:8001,http://localhost:8002")
	pingFreq := flag.String("pingfreq", "15", "Frequency of pinging peers in seconds")
	timeout := flag.String("timeout", "20", "Timeout for operations in seconds")
	flag.Parse()

	if *port == "" {
		log.Fatal("Port must be provided")
	}

	ping, err := strconv.Atoi(*pingFreq)
	if err != nil {
		log.Fatalf("Invalid ping frequency: %v", err)
	}

	time, err := strconv.Atoi(*timeout)
	if err != nil {
		log.Fatalf("Invalid timeout value: %v", err)
	}

	return &Config{
		Port:          *port,
		Peers:         strings.Split(*peers, ","),
		PingFrequency: ping,
		Timeout:       time,
	}
}
