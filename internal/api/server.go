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

func StartRegistrationServer(port string) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("host")
		ip := strings.Split(r.RemoteAddr, ":")[0]

		savePending(hostname, ip)

		fmt.Printf("\n[!] New Request: Hostname [%s] at IP [%s]", hostname, ip)
		fmt.Printf("\nType: sentinel accept %s\n> ", ip)
	})

	fmt.Printf("[*] Jumpbox Daemon listening on port %s...\n", port)
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
	var autoHostname string
	var newPending []HostEntry
	found := false

	for _, h := range pending.Hosts {
		if h.IP == childIP {
			autoHostname = h.Name
			found = true
		} else {
			newPending = append(newPending, h)
		}
	}

	if !found {
		fmt.Printf("[!] Error: No pending request found for IP: %s\n", childIP)
		return
	}

	// --- ALIAS SETUP ---
	fmt.Printf("[?] Enter a custom alias for this host (Default: %s): ", autoHostname)
	var alias string
	fmt.Scanln(&alias)
	if alias == "" {
		alias = autoHostname
	}

	pubKey, err := os.ReadFile("/etc/sentinelx/id_rsa.pub")
	if err != nil {
		fmt.Println("[!] Error: Public key not found at /etc/sentinelx/id_rsa.pub")
		return
	}

	url := fmt.Sprintf("http://%s:9091/finalize", childIP)
	resp, err := http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("[!] Failed to push key to %s. Handshake failed.\n", childIP)
		return
	}

	finalInv := loadFile(InventoryPath)
	finalInv.Hosts = append(finalInv.Hosts, HostEntry{Name: alias, IP: childIP})
	
	writeData(InventoryPath, finalInv)
	writeData(PendingPath, Inventory{Hosts: newPending})

	fmt.Printf("[+] Success! Host '%s' (%s) is ready for commands.\n", alias, childIP)
}

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

func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	
	fmt.Println("[*] Requesting connection to Jumpbox...")
	_, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("[!] Connection failed: %v\n", err)
		return
	}

	mux := http.NewServeMux()
	server := &http.Server{Addr: ":9091", Handler: mux}

	mux.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := io.ReadAll(r.Body)
		os.MkdirAll("/home/sentinelx/.ssh", 0700)
		os.WriteFile("/home/sentinelx/.ssh/authorized_keys", key, 0600)
		fmt.Println("\n[+] Key received! Trust established.")
		w.WriteHeader(http.StatusOK)
		go func() { server.Close() }()
	})
	
	fmt.Println("[*] Awaiting administrator approval...")
	server.ListenAndServe()
}
