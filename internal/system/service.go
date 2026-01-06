package system

import (
	"os"
	"os/exec"
)

func EnableBackgroundService() {
	serviceFile := `[Unit]
Description=Sentinel-X Security Agent
After=network.target

[Service]
ExecStart=/usr/local/bin/sentinel daemon
Restart=always
User=root

[Install]
WantedBy=multi-user.target`

	os.WriteFile("/etc/systemd/system/sentinel.service", []byte(serviceFile), 0644)
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "sentinel").Run()
	exec.Command("systemctl", "start", "sentinel").Run()
}
