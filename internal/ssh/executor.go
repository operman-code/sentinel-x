func ExecuteRemote(ip, command string) {
    // ... (SSH Dialing Logic) ...

    session, _ := client.NewSession()
    defer session.Close()

    // These three lines make the response "Immediate"
    session.Stdout = os.Stdout
    session.Stderr = os.Stderr
    session.Stdin = os.Stdin

    fmt.Printf("--- Output from %s ---\n", ip)
    err := session.Run(command)
    if err != nil {
        fmt.Printf("Command failed: %v\n", err)
    }
}
