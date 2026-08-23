package fixture

import . "os/exec"

func useDot() {
	_ = Command("ls")
}
