package fixture

import "os/exec"

func rawCommand() {
	_ = exec.Command("ls")
}
