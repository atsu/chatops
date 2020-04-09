package bot

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncatePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		max  int
		want string
	}{
		{"no change", "/testing", 100, "/testing"},
		{"relative", "one/one/two/three/four/file", 10, "one/*/file"},
		{"absolute", "/two/three/four/file", 16, "/two/*/file"},
		{"long file", "/fileonetwothreetest", 5, "*test"},
		{"negative max", "/fileonetwothreetest", -4, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := TruncatePath(test.path, test.max)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
	}{
		{"no error", ""},
		{"error", "failed!"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			theTpl := `{{ if .IsErr }}{{ Error .Msg }}{{ else }}success{{ end }}`

			tpl, err := template.New("test").Funcs(template.FuncMap{
				"Error": Error,
			}).Parse(theTpl)
			if err != nil {
				t.Fatal(err)
			}
			buf := new(bytes.Buffer)
			err = tpl.Execute(buf, struct {
				IsErr bool
				Msg   string
			}{
				IsErr: test.msg != "",
				Msg:   test.msg,
			})
			if err != nil {
				assert.True(t, strings.HasSuffix(err.Error(), test.msg))
			} else {
				assert.Equal(t, "success", buf.String())
			}
		})
	}
}
