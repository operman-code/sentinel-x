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

// --- KEY GENERATION ---

func GenerateMasterKeys() {
	// Create directory if it doesn't exist
	os.MkdirAll("/etc/sentinelx", 0700)

	reader := rand.Reader
	bitSize := 2048
	key, _ := rsa.GenerateKey(reader, bitSize)

	// Save Private Key
	privFile, _ := os.Create("/etc/sentinelx/id_rsa")
	defer privFile.Close()
	var privBlock = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	pem.Encode(privFile, privBlock)
	os.Chmod("/etc/sentinelx/id_rsa", 0600)

	// Save Public Key
	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	os.WriteFile("/etc/sentinelx/id_rsa.pub", pubBytes, 0644)
}

// --- COMMAND EXECUTION ---

func ExecuteRemote(target, command string) {
	targetIP := target

	// 1. ALIAS LOOKUP: Check hosts.yml for the name
	inventory := loadInventory()
	for _, host := range inventory.Hosts {
		if host.Name == target {
			targetIP = host.IP
			fmt.Printf("[*] Using Alias: %s (%s)\n", target, targetIP)
			break
		}
	}

	// 2. SSH AUTHENTICATION
	key, err := os.ReadFile("/etc/sentinelx/id_rsa")
	if err != nil {
		fmt.Println("[!] Error: Private key not found. Run 'install' first.")
		return
	}

	signer, _ := ssh.ParsePrivateKey(key)
	config := &ssh.ClientConfig{
		User: "sentinelx",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// 3. ESTABLISH CONNECTION
	client, err := ssh.Dial("tcp", targetIP+":22", config)
	if err != nil {
		fmt.Printf("[!] Connection failed to %s: %v\n", targetIP, err)
		return
	}
	defer client.Close()

	session, _ := client.NewSession()
	defer session.Close()

	// 4. IMMEDIATE REDIRECTION: Link remote pipes to local terminal
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	fmt.Printf("--- Sentinel-X: Executing on %s ---\n", target)
	
	// Start the command
	err = session.Run(command)
	if err != nil {
		fmt.Printf("[!] Command execution error: %v\n", err)
	}
}

// --- INVENTORY HELPERS ---

type HostEntry struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

type Inventory struct {
	Hosts []HostEntry `yaml:"hosts"`
}

func loadInventory() Inventory {
	var inv Inventory
	data, err := os.ReadFile("hosts.yml")
	if err != nil {
		return inv
	}
	yaml.Unmarshal(data, &inv)
	return inv
}

func ListHosts() {
	inv := loadInventory()
	fmt.Printf("%-20s %-15s\n", "ALIAS", "IP ADDRESS")
	fmt.Println(strings.Repeat("-", 35))
	for _, h := range inv.Hosts {
		fmt.Printf("%-20s %-15s\n", h.Name, h.IP)
	}
}
