package shell

import "strings"

// scriptArgv slashes drive-letter args so msys and Go agree on path spelling.
func scriptArgv(goos, bashPath, name string, args []string) (string, []string) {
	if goos != "windows" || !strings.HasSuffix(name, ".sh") {
		return name, args
	}
	rewritten := make([]string, 0, len(args)+1)
	rewritten = append(rewritten, strings.ReplaceAll(name, `\`, "/"))
	for _, arg := range args {
		rewritten = append(rewritten, slashDrivePath(arg))
	}
	return bashPath, rewritten
}

// slashDrivePath avoids filepath.ToSlash, a no-op on slash platforms, so it behaves identically everywhere.
func slashDrivePath(arg string) string {
	if len(arg) > 2 && arg[1] == ':' && (arg[2] == '\\' || arg[2] == '/') {
		return strings.ReplaceAll(arg, `\`, "/")
	}
	return arg
}
