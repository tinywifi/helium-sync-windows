package heliumsync

import (
	"net"
	"os"
	"os/exec"
	"strings"
)

func Hostname() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		addrs, _ := net.InterfaceAddrs()
		if len(addrs) > 0 {
			return "unknown"
		}
	}
	return host
}

var heliumRunningFunc = detectHeliumRunning

func HeliumRunning() bool {
	return heliumRunningFunc()
}

func detectHeliumRunning() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq helium.exe", "/NH")
	out, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "helium.exe")
}

func isTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
