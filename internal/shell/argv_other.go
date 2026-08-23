//go:build !windows

package shell

func platformArgv(name string, args []string) (string, []string) {
	return name, args
}
