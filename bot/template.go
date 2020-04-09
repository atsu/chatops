package bot

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

func ReadOnDemandTemplate(data io.Reader, tpl *template.Template) (*template.Template, *TemplateMetadata, error) {
	rawtpl, err := ioutil.ReadAll(data)
	if err != nil {
		return nil, nil, err
	}
	buf := bytes.NewBuffer(rawtpl)
	meta, err := ParseTemplateMetadata(buf)
	if err != nil {
		return nil, nil, err
	}

	fmt.Println("Parsing Text:")
	fmt.Println(string(rawtpl))
	if tpl == nil {
		tpl = template.New(meta.Name)
	} else {
		tpl = tpl.New(meta.Name)
	}
	// add global helper functions...
	if _, err = tpl.Funcs(template.FuncMap{
		"TrimPrefix":   strings.TrimPrefix,
		"CachedByKey":  CachedByKey,
		"CachedByVal":  CachedByVal,
		"GetMounts":    GetMounts,
		"GetAnomalies": GetAnomalies,
		"TruncPath":    TruncatePath,
		"Error":        Error,
	}).Parse(string(rawtpl)); err != nil {
		return nil, nil, fmt.Errorf("failed to parse template data %s", err)
	}
	return tpl, meta, nil
}

func ReadTemplates(dir string) (*template.Template, map[string]*TemplateMetadata, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	tmeta := make(map[string]*TemplateMetadata)
	tpl := template.New(dir)
	for _, f := range files {
		filename := path.Join(dir, f.Name())

		if m, err := ParseTemplateMetadataFile(filename); err != nil {
			return nil, nil, fmt.Errorf("failed to get template meatadat: %v", err)
		} else {
			tmeta[f.Name()] = m
		}

		// add global helper functions...
		if _, err = tpl.Funcs(template.FuncMap{
			"TrimPrefix":   strings.TrimPrefix,
			"CachedByKey":  CachedByKey,
			"CachedByVal":  CachedByVal,
			"GetMounts":    GetMounts,
			"GetAnomalies": GetAnomalies,
			"TruncPath":    TruncatePath,
			"Error":        Error,
		}).ParseFiles(filename); err != nil {
			return nil, nil, fmt.Errorf("failed to parse: %s", f.Name())
		}
	}
	return tpl, tmeta, nil
}

// TemplateMetadata encapsulates values passed from the template via ParseTemplateMetadataFile
type TemplateMetadata struct {
	Name             string
	Description      string
	SendToKafka      bool
	KafkaMessageType string
	IsTerminating    bool
	Dialog           bool
	Extra            map[string]interface{}
}

// ParseTemplateMetadataFile reads the template file, extracts the yml at the top into a
// struct. Valid templates should have --- marked yml eg
// ---
// myfield: asdf
// mysecondfield: 123
// ---
func ParseTemplateMetadataFile(file string) (*TemplateMetadata, error) {
	f, err := os.OpenFile(file, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseTemplateMetadata(f)
}

func ParseTemplateMetadata(data io.Reader) (*TemplateMetadata, error) {
	buf := new(bytes.Buffer)
	save, done := false, false
	sc := bufio.NewScanner(data)
	for sc.Scan() {
		line := sc.Text() // GET the line string
		if strings.TrimSpace(line) == "---" {
			save = !save
		}
		if save {
			buf.Write(sc.Bytes())
			buf.WriteString("\n") // sc.Text() strips \n so we add it back
			done = true
		} else if done {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	var m *TemplateMetadata
	if err := yaml.Unmarshal(buf.Bytes(), &m); err != nil {
		return nil, err
	}
	return m, nil
}
