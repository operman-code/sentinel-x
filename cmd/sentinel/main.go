package main

import (
	"fmt"
	"os"
	"sentinelx/internal/api"
	"sentinelx/internal/ssh"
	"sentinelx/internal/system" // New package for OS tasks
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sentinel [install | daemon | pending | accept <IP> | list | run <IP/Alias> <cmd>]")
		return
	}

	switch os.Args[1] {
	case "install":
		runWizard()
	case "daemon":
		api.StartRegistrationServer("9090")
	case "pending":
		api.ListPending()
	case "accept":
		if len(os.Args) < 3 { fmt.Println("Provide IP"); return }
		api.AcceptHost(os.Args[2])
	case "list":
		ssh.ListHosts() // Reads from hosts.yml
	case "run":
		if len(os.Args) < 3 { fmt.Println("Usage: run <IP> <cmd>"); return }
		// This now streams output immediately to your screen
		ssh.ExecuteRemote(os.Args[2], os.Args[3])
	}
}

func runWizard() {
	fmt.Println("🛡️ Sentinel-X Setup")
	fmt.Println("1) Jumpbox (Full Management)\n2) Child (Thin Agent)")
	var choice int
	fmt.Scanln(&choice)

	if choice == 1 {
		// JUMPBOX ROLE
		ssh.GenerateMasterKeys()
		// Initialize the empty inventory file
		os.WriteFile("hosts.yml", []byte("hosts: []\n"), 0644)
		fmt.Println("[+] Jumpbox Ready. Inventory created: hosts.yml")
		fmt.Println("[+] Run 'sentinel daemon' to listen for children.")
	} else {
		// THIN CHILD ROLE
		system.CreateSentinelUser() // Setup Linux user & SSH folder
		fmt.Print("Enter Jumpbox IP: ")
		var jip string
		fmt.Scanln(&jip)
		api.SendRequest(jip)
		fmt.Println("[+] Child is now waiting for Jumpbox approval...")
	}
}
