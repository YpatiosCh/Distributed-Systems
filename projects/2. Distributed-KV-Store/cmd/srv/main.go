package main

import (
	"fmt"

	"github.com/YpatiosCh/Distributed-Systems/projects/Distributed-kv-store/config"
	"github.com/YpatiosCh/Distributed-Systems/projects/Distributed-kv-store/node"
)

func main() {
	cfg := config.Load()

	node := node.NewNode(*cfg)

	fmt.Println("Node initialized with the following configuration:")
	fmt.Printf("Port: %s\n", node.Port)
	fmt.Printf("Peers: %v\n", node.Peers)
	fmt.Printf("Store: %v\n", node.DB)
	fmt.Println("Ping Frequency:", node.PingFrequency)
	fmt.Println("Timeout:", node.Timeout)

	node.Start()
}
