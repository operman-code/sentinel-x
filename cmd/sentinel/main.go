package main

import (
	"fmt"
	"os"
	"sentinelx/internal/api"
	"sentinelx/internal/ssh"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sentinel [install | daemon | pending | accept <IP> | run <IP> <cmd>]")
		return
	}

	switch os.Args[1] {
	case "install":
		runWizard()
	case "daemon":
		// Jumpbox: Start the listener for new agents
		api.StartRegistrationServer("9090")
	case "pending":
		api.ListPending()
	case "accept":
		if len(os.Args) < 3 { fmt.Println("Provide IP"); return }
		api.AcceptHost(os.Args[2])
	case "run":
		// sentinel run 10.0.0.5 "uptime"
		ssh.ExecuteRemote(os.Args[2], os.Args[3])
	}
}

func runWizard() {
	fmt.Println("Choose Role: (1) Jumpbox  (2) Child")
	var choice int
	fmt.Scanln(&choice)

	if choice == 1 {
		ssh.GenerateMasterKeys() // Create RSA keys for the Jumpbox
		fmt.Println("[+] Jumpbox Ready. Run 'sentinel daemon' to wait for requests.")
	} else {
		fmt.Print("Enter Jumpbox IP: ")
		var jip string
		fmt.Scanln(&jip)
		api.SendRequest(jip) // Child calls home
	}
}
