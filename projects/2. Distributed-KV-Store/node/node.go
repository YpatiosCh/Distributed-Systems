package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/YpatiosCh/Distributed-Systems/projects/Distributed-kv-store/config"
	"github.com/YpatiosCh/Distributed-Systems/projects/Distributed-kv-store/store"
)

// Node represents a node in the distributed key-value store.
// It contains the port on which the node listens, a list of peer nodes,
// and a store that holds key-value pairs.
// It also includes a PingFrequency for pinging peers and a Timeout for operations.
// PeerStates is a map that tracks the state of each peer (up/down).
type Node struct {
	Port          string
	Peers         []string
	PeerStates    map[string]bool
	DB            store.LocalDB
	PingFrequency int
	Timeout       int
}

// NewNode creates a new Node instance with the specified port and peers.
// It initializes the store and sets up the HTTP handler.
func NewNode(cfg config.Config) *Node {

	peerState := make(map[string]bool)
	for _, peer := range cfg.Peers {
		peerState[peer] = false // Initialize all peers as down
	}

	node := &Node{
		Port:          cfg.Port,
		Peers:         cfg.Peers,
		PeerStates:    peerState,
		DB:            store.LocalDB{}, // Initialize with an empty store
		PingFrequency: cfg.PingFrequency,
		Timeout:       cfg.Timeout,
	}

	return node
}

func (n *Node) Start() {
	// Set up HTTP server and routes
	http.HandleFunc("/ping", n.Pong)
	http.HandleFunc("/store", n.StoreKeyValue)
	http.HandleFunc("/replicate", n.ReplicateKeyValue)
	http.HandleFunc("/store/hash", n.StoreHash)
	http.HandleFunc("/store/key", n.GetValue)
	http.HandleFunc("/replicateAll", n.AcceptReplicateAll)

	go n.PingPeers()

	log.Printf("Starting node on port %s with peers: %v", n.Port, n.Peers)
	if err := http.ListenAndServe(":"+n.Port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Pong handles the ping request from peers.
// It responds with a JSON message indicating the node is alive.
func (n *Node) Pong(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// respond with json {"message": "pong"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "pong"}`))
}

// PingPeers will ping peers every node.PingFrequency seconds and expect a pong response
// if a peer does not respond within node.Timeout seconds, it will be considered down
func (n *Node) PingPeers() {
	// ticker to ping peers at regular intervals
	ticker := time.NewTicker(time.Duration(n.PingFrequency) * time.Second)
	defer ticker.Stop()

	// http client with timeout
	client := &http.Client{
		Timeout: time.Duration(n.Timeout) * time.Second,
	}

	// Track if "All peers are up" has been logged
	alreadyLoggedAllUp := false

	// Track if we've logged each peer coming up
	peerLoggedUp := make(map[string]bool)

	for range ticker.C {
		// Flag to track if all peers are up
		allUp := true

		// Iterate over peers and ping them
		for _, peer := range n.Peers {
			resp, err := client.Get(peer + "/ping")
			if err != nil {
				log.Printf("Peer %s is down: %v", peer, err)
				n.PeerStates[peer] = false
				peerLoggedUp[peer] = false // Reset log flag when peer goes down
				allUp = false              // Mark as false if any peer is down
				continue
			}

			// Manually close the response body after using it
			if resp != nil {
				// Close the body explicitly after it's used
				err := resp.Body.Close()
				if err != nil {
					log.Printf("Error closing response body for peer %s: %v", peer, err)
				}
			}

			// If response is successful, mark the peer as up
			if resp.StatusCode == http.StatusOK {
				n.PeerStates[peer] = true
				// Check if the peer was previously down and log it once
				if !peerLoggedUp[peer] {
					log.Printf("Peer %s is up", peer)
					peerLoggedUp[peer] = true // Mark as logged

					// compute the hash of the local store
					localstoreHash, err := n.computeHash()
					if err != nil {
						log.Printf("Failed to compute local store hash: %v", err)
						continue
					}

					// get the peer's store hash
					// this is used to check if the local store matches the peer's store
					peerStoreHash, err := n.getPeerStoreHash(peer)
					if err != nil {
						log.Printf("Failed to get peer store hash for %s: %v", peer, err)
						continue
					}

					// compare the local store hash with the peer's store hash
					// if they do not match, replicate the local store to the peer
					if localstoreHash != peerStoreHash {
						log.Printf("Local store hash %s does not match peer %s hash %s, replicating store", localstoreHash, peer, peerStoreHash)
						if err := n.replicateStoreToPeer(peer); err != nil {
							log.Printf("Failed to replicate store to peer %s: %v", peer, err)
						}
					} else {
						log.Printf("Local store hash matches peer %s, no replication needed", peer)
					}
				}
			} else {
				log.Printf("Peer %s responded with status: %d", peer, resp.StatusCode)
				allUp = false // Mark as false if any peer is not OK
			}
		}

		// After checking all peers, if they are all up and this message hasn't been logged, log it
		if allUp && !alreadyLoggedAllUp {
			log.Println("All peers are up")
			alreadyLoggedAllUp = true // Ensure we only log it once
		}

		// If any peer is down, reset the flag for the next cycle
		if !allUp {
			alreadyLoggedAllUp = false
		}
	}
}

// StoreKeyValue stores a key-value pair in the node's local store.
// It also replicates the key-value pair to all peers.
// It expects a POST request with a JSON body containing the key and value.
func (n *Node) StoreKeyValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the key and value from the request body
	var keyValue struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	// Decode the JSON request body into the keyValue struct
	if err := json.NewDecoder(r.Body).Decode(&keyValue); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store the key-value pair in the local store
	newStore := store.Store{
		Key:   keyValue.Key,
		Value: keyValue.Value,
	}

	n.DB.Store = append(n.DB.Store, newStore)
	fmt.Println("STORE: ", n.DB)

	// Replicate the key-value pair to peers
	for _, peer := range n.Peers {
		// Re-encode the request body for replication to peers
		body, err := json.Marshal(keyValue)
		if err != nil {
			log.Printf("Failed to marshal key-value pair: %v", err)
			continue
		}

		// Create a new request to send to the peer for replication
		resp, err := http.Post(peer+"/replicate", "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Failed to replicate to peer %s: %v", peer, err)
			continue
		}

		// Ensure that the response body is closed
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to store key-value pair on peer %s: %d", peer, resp.StatusCode)
		} else {
			log.Printf("Successfully stored key-value pair on peer %s", peer)
		}
	}
}

// ReplicateKeyValue is a method to accept replication of kv pair from a peer.
// It expects a POST request with a JSON body containing the key and value.
// it is used so all nodes can have the same key-value pairs.
func (n *Node) ReplicateKeyValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the key and value from the request body
	var keyValue store.Store
	if err := json.NewDecoder(r.Body).Decode(&keyValue); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store the key-value pair in the local store
	n.DB.Store = append(n.DB.Store, keyValue)
	fmt.Println("Received and stored key-value pair:", keyValue)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Key-value pair replicated successfully"}`))
}

func (n *Node) StoreHash(w http.ResponseWriter, r *http.Request) {
	hash, err := n.computeHash()
	if err != nil {
		http.Error(w, "Failed to compute hash", http.StatusInternalServerError)
		return
	}

	// repond with the hash
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"hash": "%s"}`, hash)))
}

func (n *Node) computeHash() (string, error) {
	storeData, err := json.Marshal(n.DB.Store)
	if err != nil {
		return "", err
	}

	// Create a SHA-256 hash of the store data
	hash := sha256.New()
	hash.Write(storeData)

	// Return the hash as a hexadecimal string
	// fmt.Sprintf is used to format the hash as a hexadecimal string
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (n *Node) getPeerStoreHash(peer string) (string, error) {
	resp, err := http.Get(peer + "/store/hash")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode peer store hash: %w", err)
	}

	return response.Hash, nil
}

func (n *Node) replicateStoreToPeer(peer string) error {
	// marshal the local store to JSON
	storeData, err := json.Marshal(n.DB.Store)
	if err != nil {
		return fmt.Errorf("failed to marshal local store: %w", err)
	}

	// send the entire store to the peer
	resp, err := http.Post(peer+"/replicateAll", "application/json", bytes.NewBuffer(storeData))
	if err != nil {
		return fmt.Errorf("failed to replicate store to peer %s: %w", peer, err)
	}
	defer resp.Body.Close()

	// check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to replicate store to peer %s: received status code %d", peer, resp.StatusCode)
	}

	// log success
	log.Printf("Successfully replicated store to peer %s", peer)
	return nil
}

// AcceptReplicateAll accepts a replication of the entire store from a peer.
// It expects a POST request with a JSON body containing an array of key-value pairs.
func (n *Node) AcceptReplicateAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the entire store from the request body
	// This expects a JSON array of key-value pairs
	var storeData []store.Store
	if err := json.NewDecoder(r.Body).Decode(&storeData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	n.DB.Store = storeData
	fmt.Println("Received and stored all key-value pairs:", n.DB.Store)
	fmt.Println(n.DB)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "All key-value pairs replicated successfully"}`))
}

func (n *Node) GetValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the key from the request body
	var keyValue struct {
		Key string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&keyValue); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Search for the key in the local store
	for _, storeItem := range n.DB.Store {
		if storeItem.Key == keyValue.Key {
			// If found, respond with the value
			response := struct {
				Value any `json:"value"`
			}{
				Value: storeItem.Value,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
	}
	// If not found, respond with an error
	http.Error(w, "Key not found", http.StatusNotFound)
}
