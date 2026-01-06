package system

import (
	"os"
	"os/exec"
)

// CreateSentinelUser sets up the Linux environment on the Child
func CreateSentinelUser() {
	// 1. Create user 'sentinelx'
	exec.Command("useradd", "-m", "-s", "/bin/bash", "sentinelx").Run()

	// 2. Create .ssh directory
	sshPath := "/home/sentinelx/.ssh"
	os.MkdirAll(sshPath, 0700)

	// 3. Fix ownership
	exec.Command("chown", "-R", "sentinelx:sentinelx", "/home/sentinelx").Run()
}
