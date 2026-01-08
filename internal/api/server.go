package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// PendingHosts stores IP -> Hostname mapping
var (
	PendingHosts = make(map[string]string)
	mutex        = &sync.Mutex{}
)

// StartRegistrationServer runs on the Jumpbox
func StartRegistrationServer(port string) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("host")
		// r.RemoteAddr includes the port (e.g. 192.168.1.50:54321)
		// We strip the port to store just the IP for easier 'accept' commands
		ip := strings.Split(r.RemoteAddr, ":")[0]

		mutex.Lock()
		PendingHosts[ip] = hostname
		mutex.Unlock()

		fmt.Printf("\n[!] New Request: Hostname [%s] at IP [%s]", hostname, ip)
		fmt.Printf("\nType: sentinel accept %s\n> ", ip)
	})

	fmt.Printf("[*] Jumpbox Daemon listening on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}

// SendRequest is called by the Child (Thin Agent)
func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	
	fmt.Println("[*] Requesting connection to Jumpbox...")
	_, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("[!] Connection failed: %v\n", err)
		return
	}

	// Temporary listener for the RSA key push
	http.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := io.ReadAll(r.Body)
		
		// Create the directory for the sentinelx user
		os.MkdirAll("/home/sentinelx/.ssh", 0700)
		
		err := os.WriteFile("/home/sentinelx/.ssh/authorized_keys", key, 0600)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Println("[+] Key received! Trust established.")
		// We don't os.Exit(0) here if we want the daemon to keep running
		w.WriteHeader(http.StatusOK)
	})
	
	fmt.Println("[*] Awaiting administrator approval...")
	http.ListenAndServe(":9091", nil)
}

// AcceptHost maps the hostname to IP and saves it to the inventory
func AcceptHost(childIP string) {
	mutex.Lock()
	hostname, exists := PendingHosts[childIP]
	mutex.Unlock()

	if !exists {
		fmt.Printf("[!] Error: No pending request found for IP: %s\n", childIP)
		return
	}

	// 1. Read Jumpbox Public Key (Generated during install)
	pubKey, err := os.ReadFile("/etc/sentinelx/id_rsa.pub")
	if err != nil {
		fmt.Println("[!] Error: Public key not found at /etc/sentinelx/id_rsa.pub")
		return
	}

	// 2. SAVE TO HOSTS.YML (The core of your requested change)
	saveToInventory(hostname, childIP)

	// 3. Push key to Child
	url := fmt.Sprintf("http://%s:9091/finalize", childIP)
	resp, err := http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("[!] Failed to push key to %s\n", childIP)
		return
	}

	fmt.Printf("[+] Success! %s (%s) is now in your inventory.\n", hostname, childIP)
}

func saveToInventory(name, ip string) {
	// We use a simplified YAML format that is easy to append to
	entry := fmt.Sprintf("- name: %s\n  ip: %s\n", name, ip)
	
	// Create or append to hosts.yml
	f, err := os.OpenFile("hosts.yml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("[!] Error writing to hosts.yml")
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

// ListPending displays all current requests
func ListPending() {
	mutex.Lock()
	defer mutex.Unlock()
	if len(PendingHosts) == 0 {
		fmt.Println("No pending requests.")
		return
	}
	fmt.Println("Pending Host Requests:")
	for ip, name := range PendingHosts {
		fmt.Printf(" - %s (%s)\n", name, ip)
	}
}
