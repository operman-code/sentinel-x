package ssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3" // Make sure to run: go get gopkg.in/yaml.v3
)

// HostEntry defines the structure for our YAML inventory
type HostEntry struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

type Inventory struct {
	Hosts []HostEntry `yaml:"hosts"`
}

// ExecuteRemote finds the target and runs the command with immediate output
func ExecuteRemote(target, command string) {
	targetIP := target

	// 1. Try to find the IP by Alias in hosts.yml
	inventory := loadInventory()
	for _, host := range inventory.Hosts {
		if host.Name == target {
			targetIP = host.IP
			fmt.Printf("[*] Alias matched: %s -> %s\n", target, targetIP)
			break
		}
	}

	// 2. SSH Connection Setup
	// Note: We use the sentinelx user we created during install
	config := &ssh.ClientConfig{
		User: "sentinelx",
		Auth: []ssh.AuthMethod{
			// This assumes your private key is at the default Jumpbox location
			publicKeyAuth("/etc/sentinelx/id_rsa"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", targetIP+":22", config)
	if err != nil {
		fmt.Printf("[!] Failed to connect to %s: %v\n", targetIP, err)
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Printf("[!] Failed to create session: %v\n", err)
		return
	}
	defer session.Close()

	// 3. THE MAGIC: Immediate redirection to your screen
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	fmt.Printf("--- Output from %s ---\n", target)
	err = session.Run(command)
	if err != nil {
		fmt.Printf("\n[!] Command failed: %v\n", err)
	}
}

// ListHosts reads the inventory and prints it nicely
func ListHosts() {
	inventory := loadInventory()
	if len(inventory.Hosts) == 0 {
		fmt.Println("Inventory is empty. Use 'sentinel accept <IP>' to add hosts.")
		return
	}

	fmt.Println("Managed Sentinel-X Hosts:")
	fmt.Printf("%-20s %-15s\n", "HOSTNAME", "IP ADDRESS")
	fmt.Println(strings.Repeat("-", 35))
	for _, h := range inventory.Hosts {
		fmt.Printf("%-20s %-15s\n", h.Name, h.IP)
	}
}

// loadInventory helper to parse hosts.yml
func loadInventory() Inventory {
	var inv Inventory
	yamlFile, err := ioutil.ReadFile("hosts.yml")
	if err != nil {
		return inv // Return empty if file doesn't exist
	}
	yaml.Unmarshal(yamlFile, &inv)
	return inv
}

// publicKeyAuth helper for SSH authentication
func publicKeyAuth(keyPath string) ssh.AuthMethod {
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(signer)
}
