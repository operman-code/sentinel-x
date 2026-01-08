package main

import (
	"fmt"
	"os"
	"sentinelx/internal/api"
	"sentinelx/internal/ssh"
	"sentinelx/internal/system" // Package handling systemd and user creation
)

func main() {
	// If no arguments, show help
	if len(os.Args) < 2 {
		fmt.Println("Usage: sentinel [install | daemon | pending | accept <IP> | list | run <Alias/IP> <cmd>]")
		return
	}

	switch os.Args[1] {
	case "install":
		runWizard()
	case "daemon":
		// This is what the background service runs
		fmt.Println("[*] Sentinel-X Daemon is active...")
		api.StartRegistrationServer("9090")
	case "pending":
		api.ListPending()
	case "accept":
		if len(os.Args) < 3 {
			fmt.Println("Error: Provide the IP of the child to accept.")
			return
		}
		api.AcceptHost(os.Args[2])
	case "list":
		ssh.ListHosts() 
	case "run":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sentinel run <Alias/IP> \"command\"")
			return
		}
		// Alias lookup happens inside ExecuteRemote using hosts.yml
		ssh.ExecuteRemote(os.Args[2], os.Args[3])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
	}
}

func runWizard() {
	fmt.Println("🛡️ Sentinel-X Setup Wizard")
	fmt.Println("---------------------------")
	fmt.Println("1) Jumpbox (Manager Role)")
	fmt.Println("2) Child   (Thin Agent Role)")
	fmt.Print("\nSelect Role [1-2]: ")

	var choice int
	fmt.Scanln(&choice)

	if choice == 1 {
		// --- JUMPBOX SETUP ---
		fmt.Println("[*] Configuring Jumpbox...")
		ssh.GenerateMasterKeys()
		
		// Create initial empty inventory if it doesn't exist
		if _, err := os.Stat("hosts.yml"); os.IsNotExist(err) {
			os.WriteFile("hosts.yml", []byte("hosts: []\n"), 0644)
		}

		// Move binary to /usr/local/bin and enable systemd service
		system.InstallService()
		
		fmt.Println("\n[+] Jumpbox Installation Successful!")
		fmt.Println("[+] You can now run 'sentinel' from any directory.")
		
	} else if choice == 2 {
		// --- CHILD SETUP ---
		fmt.Println("[*] Configuring Thin Agent...")
		
		// 1. Prepare the OS environment
		system.CreateSentinelUser() 
		
		// 2. Install the binary as a service so it stays online
		system.InstallService()

		// 3. Link to the manager
		fmt.Print("Enter Jumpbox IP: ")
		var jip string
		fmt.Scanln(&jip)
		
		api.SendRequest(jip)
		
		fmt.Println("\n[+] Child Agent is now signaling the Jumpbox.")
		fmt.Println("[+] Once accepted, this server will be fully managed.")
	} else {
		fmt.Println("[!] Invalid selection. Exiting.")
	}
}
