package fixture

import oe "os/exec"

func useAliased() {
	_ = oe.Command("ls")
}
