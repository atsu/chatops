package util

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		output map[string]interface{}
	}{
		{"flag only",
			[]string{"-one"},
			map[string]interface{}{
				"one": true,
			},
		},
		{"no flags",
			[]string{"one"},
			map[string]interface{}{},
		},
		{"pair and flag",
			[]string{"-one", "two", "-three"},
			map[string]interface{}{
				"one":   "two",
				"three": true,
			},
		},
		{"flag and pair",
			[]string{"-one", "-two", "three"},
			map[string]interface{}{
				"one": true,
				"two": "three",
			},
		},
		{"2pair",
			[]string{"-one", "four", "-two", "three"},
			map[string]interface{}{
				"one": "four",
				"two": "three",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ParseArgs(test.input)
			assert.Equal(t, test.output, got)
		})
	}
}

func TestGetAbsoluteFilePath(t *testing.T) {
	cwd, err := os.Getwd()
	fmt.Println(cwd)
	if err != nil {
		t.FailNow()
	}
	tests := []struct {
		name string
		file string
		want string
	}{
		{"file name only", "file.txt", path.Join(cwd, "file.txt")},
		{"absolute path", "/some/path/file.txt", "/some/path/file.txt"},
		{"relative file name", "./file.txt", path.Join(cwd, "file.txt")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetAbsoluteFilePath(test.file)
			assert.Equal(t, test.want, got)
		})
	}
}
