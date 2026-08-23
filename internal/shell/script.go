package shell

import "strings"

// scriptArgv rewrites a .sh invocation to run under bash on windows and
// slashes drive-letter args so msys and Go agree on path spelling. On any
// other goos, or for non-script commands, argv passes through unchanged.
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

// slashDrivePath converts only unambiguous absolute Windows paths (C:\...).
// Everything else is opaque script input and passes through untouched. The
// conversion is explicit, not filepath.ToSlash — that is a no-op when the
// binary is compiled for a slash-separator platform, and this function must
// behave identically everywhere for the tests to mean anything.
func slashDrivePath(arg string) string {
	if len(arg) > 2 && arg[1] == ':' && (arg[2] == '\\' || arg[2] == '/') {
		return strings.ReplaceAll(arg, `\`, "/")
	}
	return arg
}
