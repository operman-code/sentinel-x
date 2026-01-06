package api

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
)

// SendRequest is called by the Child during installation
func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	http.Post(url, "text/plain", nil)
	
	// Start a temporary listener to receive the key after admin approves
	http.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := os.ReadAll(r.Body)
		os.WriteFile("/home/sentinelx/.ssh/authorized_keys", key, 0600)
		fmt.Println("[+] Key received! Trust established.")
		os.Exit(0) 
	})
	http.ListenAndServe(":9091", nil)
}

// AcceptHost is called by the Jumpbox admin
func AcceptHost(childIP string) {
	pubKey, _ := os.ReadFile("/etc/sentinelx/id_rsa.pub")
	// Send the key back to the child host's temporary listener
	http.Post("http://"+childIP+":9091/finalize", "text/plain", bytes.NewBuffer(pubKey))
}
