package bot

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadTemplatesMetadata(t *testing.T) {
	templates, templateMeta, err := ReadTemplates("testdata/templates")
	assert.NoError(t, err)

	assert.NotNil(t, templates.Lookup(t.Name()+".tpl"))

	if m, ok := templateMeta[t.Name()+".tpl"]; !ok {
		t.Fail()
	} else {
		assert.Equal(t, t.Name(), m.Name)
		assert.Equal(t, "test description", m.Description)
		assert.True(t, m.Dialog)
		assert.True(t, m.SendToKafka)
		assert.True(t, m.IsTerminating)
		assert.Equal(t, "value", m.Extra["key"])
	}
}

func TestParseTemplateMetadataFile(t *testing.T) {
	file := fmt.Sprintf("testdata/templates/%s.tpl", t.Name())
	templateMeta, err := ParseTemplateMetadataFile(file)
	assert.NoError(t, err)

	assert.NotNil(t, templateMeta)
	assert.Equal(t, t.Name(), templateMeta.Name)
	assert.Equal(t, "test description", templateMeta.Description)
	assert.True(t, templateMeta.Dialog)
	assert.True(t, templateMeta.SendToKafka)
	assert.True(t, templateMeta.IsTerminating)
	assert.Equal(t, "value", templateMeta.Extra["key"])
}
