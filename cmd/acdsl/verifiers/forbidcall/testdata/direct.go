package fixture

import "os/exec"

func useDirect() {
	_ = exec.Command("ls")
}
