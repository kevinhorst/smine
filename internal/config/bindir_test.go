package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmineBinDir(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		goos         string
		home         string
		localAppData string
	}

	tests := make([]*testCase, 0)

	// darwin-go-bin
	tests = append(tests, &testCase{
		_id:       "darwin-go-bin",
		_expected: filepath.Join("/Users/u", "go", "bin"),

		goos:         "darwin",
		home:         "/Users/u",
		localAppData: "",
	})

	// linux-go-bin
	tests = append(tests, &testCase{
		_id:       "linux-go-bin",
		_expected: filepath.Join("/home/u", "go", "bin"),

		goos:         "linux",
		home:         "/home/u",
		localAppData: "",
	})

	// windows-localappdata
	tests = append(tests, &testCase{
		_id:       "windows-localappdata",
		_expected: filepath.Join("D:", "Local", "smine", "bin"),

		goos:         "windows",
		home:         `C:\Users\u`,
		localAppData: filepath.Join("D:", "Local"),
	})

	// windows-localappdata-unset-falls-back-to-home
	tests = append(tests, &testCase{
		_id:       "windows-localappdata-unset-falls-back-to-home",
		_expected: filepath.Join(`C:\Users\u`, "AppData", "Local", "smine", "bin"),

		goos:         "windows",
		home:         `C:\Users\u`,
		localAppData: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", test.localAppData)

			assert.Equal(t, test._expected, SmineBinDir(test.goos, test.home))
		})
	}
}
