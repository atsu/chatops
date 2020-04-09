package util

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// SendResponseURL sends the provided message as the POST body
func SendResponseURL(url string, body []byte) (int, []byte, error) {
	payload := bytes.NewReader(body)
	res, err := http.Post(url, "application/json", payload)
	if err != nil {
		return 0, nil, fmt.Errorf("failed POST to %s - %s", url, err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	b, err := ioutil.ReadAll(res.Body)
	return res.StatusCode, b, err
}

func StripSlackUsers(str string) string {
	rx := regexp.MustCompile("<@[^>]+>") // Strip users...
	return strings.TrimSpace(rx.ReplaceAllString(str, ""))
}

// ParseArgs take an argument string, and convert it into a map
func ParseArgs(args []string) map[string]interface{} {
	pairs := make(map[string]interface{})
	flg := ""
	argcnt := len(args)
	for i := 0; i < argcnt; i++ {
		if strings.HasPrefix(args[i], "-") {
			flg = strings.Trim(args[i], "-")
		}

		// last arg
		if i >= argcnt-1 {
			if flg != "" {
				pairs[flg] = true
				flg = ""
			}
		} else {
			next := args[i+1]
			if strings.HasPrefix(next, "-") {
				pairs[flg] = true
			} else {
				pairs[flg] = strings.TrimPrefix(next, "-")
				i++
			}
			flg = ""
		}
	}
	return pairs
}

func ComputeSha256HMAC(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}

func DecodePayloadBody(body []byte) ([]byte, error) {
	payload := bytes.TrimPrefix(body, []byte("payload="))
	jsonStr, err := url.QueryUnescape(string(payload))
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

// FindTemplate takes in arguments, attempts to find the associated template, and returns
// the args that followed the template.
// Example: input is 'describe mounts mount 123' and template 'describe_mounts.tpl' is found, that template is
// returned along with the left over arguments []string{ "mount", "123" }
func FindTemplate(templates *template.Template, args ...string) (*template.Template, []string) {
	tplname := ""
	for i := len(args); i >= 0; i-- {
		// TODO:(smt) skip args starting with '-'
		name := strings.Join(args[0:i], "_")
		tplname = fmt.Sprintf("%s.tpl", name)
		tpl := templates.Lookup(tplname)
		if tpl != nil {
			return tpl, args[i:]
		}
	}
	return nil, args
}

func GetAbsoluteFilePath(file string) string {
	if !filepath.IsAbs(file) {
		cd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		return path.Join(cd, file)
	}
	return file
}
