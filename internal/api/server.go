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
	// NEW: Wipe pending requests on startup to clear any past "dead" sessions
	fmt.Println("[*] System Start: Cleaning up old pending requests...")
	writeData(PendingPath, Inventory{Hosts: []HostEntry{}})
	
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("host")
		ip := strings.Split(r.RemoteAddr, ":")[0]

		savePending(hostname, ip)

		fmt.Printf("\n[!] New Registration Request: %s (%s)", hostname, ip)
		fmt.Printf("\nAction required: sudo sentinel accept %s\n> ", ip)
	})

	fmt.Printf("[*] Sentinel-X Jumpbox Service listening on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}

func savePending(name, ip string) {
	inv := loadFile(PendingPath)
	for _, h := range inv.Hosts {
		if h.IP == ip { return }
	}
	inv.Hosts = append(inv.Hosts, HostEntry{Name: name, IP: ip})
	writeData(PendingPath, inv)
}

func AcceptHost(childIP string) {
	pending := loadFile(PendingPath)
	inventory := loadFile(InventoryPath)
	
	var targetEntry HostEntry
	var newPending []HostEntry
	found := false

	// Find the host in pending list
	for _, h := range pending.Hosts {
		if h.IP == childIP {
			targetEntry = h
			found = true
		} else {
			newPending = append(newPending, h)
		}
	}

	if !found {
		fmt.Printf("[!] Error: IP %s is not currently requesting registration.\n", childIP)
		return
	}

	// 1. Alias Selection
	fmt.Printf("[?] Enter custom alias for %s (Default: %s): ", childIP, targetEntry.Name)
	var alias string
	fmt.Scanln(&alias)
	if alias == "" {
		alias = targetEntry.Name
	}

	// 2. Load Public Key from the vault
	pubKey, err := os.ReadFile("/etc/sentinelx/id_rsa.pub")
	if err != nil {
		fmt.Println("[!] Critical Error: Public key not found at /etc/sentinelx/id_rsa.pub")
		return
	}

	// 3. Push Key to Child (The Handshake)
	url := fmt.Sprintf("http://%s:9091/finalize", childIP)
	resp, err := http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("[!] Handshake failed with %s. Is the child agent running?\n", childIP)
		return
	}

	// 4. Update Inventory and Clear Pending
	inventory.Hosts = append(inventory.Hosts, HostEntry{Name: alias, IP: childIP})
	writeData(InventoryPath, inventory) 
	writeData(PendingPath, Inventory{Hosts: newPending}) 

	fmt.Printf("[+] Success! %s (%s) is now in the active inventory.\n", alias, childIP)
}

func ListPending() {
	inv := loadFile(PendingPath)
	if len(inv.Hosts) == 0 {
		fmt.Println("No active registration requests.")
		return
	}
	fmt.Println("Current Pending Requests:")
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
	// Ensure the directory exists before writing
	os.MkdirAll("/etc/sentinelx", 0755)
	os.WriteFile(path, data, 0644)
}

// SendRequest runs on the Child Node
func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	
	fmt.Printf("[*] Sending registration request to Jumpbox (%s)...\n", jumpboxIP)
	_, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("[!] Could not connect to Jumpbox: %v\n", err)
		return
	}

	mux := http.NewServeMux()
	server := &http.Server{Addr: ":9091", Handler: mux}

	mux.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := io.ReadAll(r.Body)
		os.MkdirAll("/home/sentinelx/.ssh", 0700)
		os.WriteFile("/home/sentinelx/.ssh/authorized_keys", key, 0600)
		fmt.Println("\n[+] Success! Master key received. Management active.")
		w.WriteHeader(http.StatusOK)
		go func() { server.Close() }() 
	})
	
	fmt.Println("[*] Awaiting administrator approval on Jumpbox...")
	server.ListenAndServe()
}
