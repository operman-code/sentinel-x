package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"os"
)

// GenerateMasterKeys creates the RSA pair the Jumpbox uses to control children
func GenerateMasterKeys() {
	reader := rand.Reader
	bitSize := 2048
	key, _ := rsa.GenerateKey(reader, bitSize)

	// Save Private Key
	privFile, _ := os.Create("/etc/sentinelx/id_rsa")
	var privBlock = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	pem.Encode(privFile, privBlock)

	// Save Public Key (the part we send to children)
	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	os.WriteFile("/etc/sentinelx/id_rsa.pub", pubBytes, 0644)
}

// ExecuteRemote runs a command on a child without a manual SSH connection
func ExecuteRemote(ip, command string) {
	// 1. Load private key
	// 2. ssh.Dial(ip)
	// 3. session.Run(command)
	// (See previous code snippets for the full 7-step dial logic)
}
