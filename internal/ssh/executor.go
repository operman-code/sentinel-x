package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// --- DATA MODELS ---

type HostEntry struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

type Inventory struct {
	Hosts []HostEntry `yaml:"hosts"`
}

// --- KEY GENERATION ---

// GenerateMasterKeys creates the RSA pair for the Jumpbox in /etc/sentinelx
func GenerateMasterKeys() {
	os.MkdirAll("/etc/sentinelx", 0700)

	reader := rand.Reader
	bitSize := 2048
	key, _ := rsa.GenerateKey(reader, bitSize)

	// 1. Save Private Key
	privFile, _ := os.Create("/etc/sentinelx/id_rsa")
	defer privFile.Close()
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	pem.Encode(privFile, privBlock)
	os.Chmod("/etc/sentinelx/id_rsa", 0600)

	// 2. Save Public Key
	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	os.WriteFile("/etc/sentinelx/id_rsa.pub", pubBytes, 0644)
	
	fmt.Println("[+] Master SSH Keys generated successfully.")
}

// --- REMOTE EXECUTION ---

// ExecuteRemote resolves Alias -> IP and runs the command with real-time output
func ExecuteRemote(target, command string) {
	targetIP := target

	// 1. Resolve Alias from hosts.yml
	inventory := loadInventory()
	for _, host := range inventory.Hosts {
		if host.Name == target {
			targetIP = host.IP
			fmt.Printf("[*] Alias Matched: %s -> %s\n", target, targetIP)
			break
		}
	}

	// 2. Load Private Key for Auth
	keyBytes, err := os.ReadFile("/etc/sentinelx/id_rsa")
	if err != nil {
		fmt.Println("[!] Error: Private key not found. Did you run 'install'?")
		return
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		fmt.Printf("[!] Error parsing private key: %v\n", err)
		return
	}

	config := &ssh.ClientConfig{
		User: "sentinelx",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// 3. Connect to Remote Child
	client, err := ssh.Dial("tcp", targetIP+":22", config)
	if err != nil {
		fmt.Printf("[!] Connection failed to %s: %v\n", targetIP, err)
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Printf("[!] Failed to create SSH session: %v\n", err)
		return
	}
	defer session.Close()

	// 4. THE MAGIC: Redirect pipes for live streaming
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	fmt.Printf("--- Sentinel-X: %s ---\n", target)
	if err := session.Run(command); err != nil {
		fmt.Printf("\n[!] Execution Error: %v\n", err)
	}
}

// --- INVENTORY HELPERS ---

// ListHosts prints a clean table of managed servers
func ListHosts() {
	inv := loadInventory()
	if len(inv.Hosts) == 0 {
		fmt.Println("No hosts in inventory.")
		return
	}

	fmt.Printf("%-20s %-15s\n", "ALIAS/NAME", "IP ADDRESS")
	fmt.Println(strings.Repeat("-", 35))
	for _, h := range inv.Hosts {
		fmt.Printf("%-20s %-15s\n", h.Name, h.IP)
	}
}

// loadInventory reads and parses the hosts.yml file
func loadInventory() Inventory {
	var inv Inventory
	data, err := os.ReadFile("hosts.yml")
	if err != nil {
		return inv // Return empty if file missing
	}
	yaml.Unmarshal(data, &inv)
	return inv
}
