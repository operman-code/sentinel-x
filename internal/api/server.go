package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// PendingHosts stores IP -> Hostname mapping temporarily until 'accept' is called
var (
	PendingHosts = make(map[string]string)
	mutex        = &sync.Mutex{}
)

// StartRegistrationServer runs on the Jumpbox to listen for "Hello" from children
func StartRegistrationServer(port string) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("host")
		ip := r.RemoteAddr // Format usually is "192.168.1.50:port"

		mutex.Lock()
		PendingHosts[ip] = hostname
		mutex.Unlock()

		fmt.Printf("\n[!] New Request: Hostname [%s] at IP [%s]", hostname, ip)
		fmt.Print("\nType 'sentinel accept <IP>' to authorize.\n> ")
	})

	fmt.Printf("[*] Jumpbox Daemon started on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}

// SendRequest is called by the Child during installation
func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	
	fmt.Println("[*] Contacting Jumpbox...")
	_, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("[!] Connection failed: %v\n", err)
		return
	}

	// Start a temporary listener to receive the RSA key back from Jumpbox
	http.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := io.ReadAll(r.Body)
		
		// Ensure the .ssh folder exists for the sentinelx user
		os.MkdirAll("/home/sentinelx/.ssh", 0700)
		
		err := os.WriteFile("/home/sentinelx/.ssh/authorized_keys", key, 0600)
		if err != nil {
			fmt.Println("[!] Failed to save key:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Println("[+] Key received! You are now managed by the Jumpbox.")
		os.Exit(0) // Child setup is complete
	})
	
	fmt.Println("[*] Waiting for Jumpbox administrator to 'accept' this host...")
	http.ListenAndServe(":9091", nil)
}

// AcceptHost is called by the Jumpbox admin to finalize trust and save to inventory
func AcceptHost(childIP string) {
	mutex.Lock()
	hostname, exists := PendingHosts[childIP]
	mutex.Unlock()

	if !exists {
		// If exact match fails, check if we need to strip the port from the IP
		fmt.Printf("[!] Error: No pending request found for %s\n", childIP)
		return
	}

	// 1. Read Jumpbox Public Key
	pubKey, err := os.ReadFile("/etc/sentinelx/id_rsa.pub")
	if err != nil {
		fmt.Println("[!] Error: Jumpbox keys not found. Run 'install' first.")
		return
	}

	// 2. Save to hosts.yml (The Inventory)
	saveToInventory(hostname, childIP)

	// 3. Send the key back to the child host
	url := fmt.Sprintf("http://%s:9091/finalize", childIP)
	_, err = http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
	if err != nil {
		fmt.Printf("[!] Failed to send key to child: %v\n", err)
		return
	}

	fmt.Printf("[+] Success! %s (%s) added to hosts.yml and authorized.\n", hostname, childIP)
}

// saveToInventory appends the host to a simple YAML-style file
func saveToInventory(name, ip string) {
	entry := fmt.Sprintf("- name: %s\n  ip: %s\n", name, ip)
	f, err := os.OpenFile("hosts.yml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("[!] Could not update hosts.yml")
		return
	}
	defer f.Close()
	f.WriteString(entry)
}
