package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"gopkg.in/yaml.v3"
)

// Define the file paths as constants so they are easy to manage
const (
	InventoryPath = "/etc/sentinelx/hosts.yml"
	PendingPath   = "/etc/sentinelx/pending_hosts.yml"
)

type HostEntry struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

type Inventory struct {
	Hosts []HostEntry `yaml:"hosts"`
}

// StartRegistrationServer runs on the Jumpbox
func StartRegistrationServer(port string) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("host")
		ip := strings.Split(r.RemoteAddr, ":")[0]

		// PERSISTENCE: Save to file so CLI can see it
		savePending(hostname, ip)

		fmt.Printf("\n[!] New Request: Hostname [%s] at IP [%s]", hostname, ip)
		fmt.Printf("\nType: sentinel accept %s\n> ", ip)
	})

	fmt.Printf("[*] Jumpbox Daemon listening on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}

// savePending writes the request to a YAML file in /etc/sentinelx
func savePending(name, ip string) {
	inv := loadFile(PendingPath)
	
	// Avoid duplicates
	for _, h := range inv.Hosts {
		if h.IP == ip { return }
	}

	inv.Hosts = append(inv.Hosts, HostEntry{Name: name, IP: ip})
	writeData(PendingPath, inv)
}

// AcceptHost reads from PendingPath and moves the entry to InventoryPath
func AcceptHost(childIP string) {
	pending := loadFile(PendingPath)
	var targetHost string
	var newPending []HostEntry
	found := false

	// Find the host in pending and remove it
	for _, h := range pending.Hosts {
		if h.IP == childIP {
			targetHost = h.Name
			found = true
		} else {
			newPending = append(newPending, h)
		}
	}

	if !found {
		fmt.Printf("[!] Error: No pending request found for IP: %s\n", childIP)
		return
	}

	// 1. Read Public Key
	pubKey, err := os.ReadFile("/etc/sentinelx/id_rsa.pub")
	if err != nil {
		fmt.Println("[!] Error: Public key not found at /etc/sentinelx/id_rsa.pub")
		return
	}

	// 2. Push key to Child
	url := fmt.Sprintf("http://%s:9091/finalize", childIP)
	resp, err := http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("[!] Failed to push key to %s. Is the child listening?\n", childIP)
		return
	}

	// 3. Move from Pending to Final Inventory
	finalInv := loadFile(InventoryPath)
	finalInv.Hosts = append(finalInv.Hosts, HostEntry{Name: targetHost, IP: childIP})
	
	writeData(InventoryPath, finalInv)   // Save to hosts.yml
	writeData(PendingPath, Inventory{Hosts: newPending}) // Update pending list

	fmt.Printf("[+] Success! %s (%s) added to inventory.\n", targetHost, childIP)
}

// ListPending reads the file so the CLI can display it
func ListPending() {
	inv := loadFile(PendingPath)
	if len(inv.Hosts) == 0 {
		fmt.Println("No pending requests.")
		return
	}
	fmt.Println("Pending Host Requests:")
	for _, h := range inv.Hosts {
		fmt.Printf(" - %s (%s)\n", h.Name, h.IP)
	}
}

// --- HELPER FUNCTIONS ---

func loadFile(path string) Inventory {
	var inv Inventory
	data, err := os.ReadFile(path)
	if err != nil { return inv }
	yaml.Unmarshal(data, &inv)
	return inv
}

func writeData(path string, inv Inventory) {
	data, _ := yaml.Marshal(inv)
	os.WriteFile(path, data, 0644)
}

// SendRequest stays largely the same, just ensure it handles the incoming key
func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	
	fmt.Println("[*] Requesting connection to Jumpbox...")
	_, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("[!] Connection failed: %v\n", err)
		return
	}

	// Simple HTTP server to receive the RSA key
	mux := http.NewServeMux()
	server := &http.Server{Addr: ":9091", Handler: mux}

	mux.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := io.ReadAll(r.Body)
		os.MkdirAll("/home/sentinelx/.ssh", 0700)
		os.WriteFile("/home/sentinelx/.ssh/authorized_keys", key, 0600)
		// Change ownership to the sentinelx user so SSH works
		fmt.Println("\n[+] Key received! Trust established.")
		w.WriteHeader(http.StatusOK)
		go func() { server.Close() }() // Close listener once key is received
	})
	
	fmt.Println("[*] Awaiting administrator approval...")
	server.ListenAndServe()
}
