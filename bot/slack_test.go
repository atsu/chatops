package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"

	"github.com/atsu/chatops/db"

	"github.com/stretchr/testify/mock"

	"github.com/atsu/chatops/interfaces/mocks"
	"github.com/atsu/chatops/util"
	gutil "github.com/atsu/goat/util"
	"github.com/nlopes/slack"
	"github.com/stretchr/testify/assert"
)

// randMap creates a generator with an internal map for generating random strings.
// Ex:
//   create the generator
//   gen := randMap()
//   then generate a value with gen(string)
//   the string will always return the same value
func randMap() func(string) string {
	m := make(map[string]string)
	return func(key string) string {
		if v, ok := m[key]; ok {
			return v
		} else {
			s := gutil.RandomString(10)
			m[key] = s
			return s
		}
	}

}

func TestSlack_ConvertBlockAction(t *testing.T) {
	rm := randMap()
	actionId := rm("Id") + "|" + rm("TemplateName")
	env := map[string]string{
		"ElasticSearchUrl": rm("ElasticSearchUrl"),
		"ViewUrl":          rm("ViewUrl"),
	}
	interaction := map[string]interface{}{
		"text":           rm("Text"),
		"value":          rm("Value"),
		"i-channel":      rm("InitialChannel"),
		"channel":        rm("SelectedChannel"),
		"i-option":       rm("InitialOption"),
		"option":         rm("SelectedOption"),
		"i-user":         rm("InitialUser"),
		"user":           rm("SelectedUser"),
		"i-date":         rm("InitialDate"),
		"date":           rm("SelectedDate"),
		"i-conversation": rm("InitialConversation"),
		"conversation":   rm("SelectedConversation"),
	}

	mockCom := new(mocks.ChatOpsCom)
	mockCom.On("EnvironmentParams").Return(env)
	ba := slack.BlockAction{
		Text:                 slack.TextBlockObject{Text: rm("Text")},
		Value:                rm("Value"),
		InitialChannel:       rm("InitialChannel"),
		SelectedChannel:      rm("SelectedChannel"),
		InitialOption:        slack.OptionBlockObject{Value: rm("InitialOption")},
		SelectedOption:       slack.OptionBlockObject{Value: rm("SelectedOption")},
		InitialUser:          rm("InitialUser"),
		SelectedUser:         rm("SelectedUser"),
		InitialDate:          rm("InitialDate"),
		SelectedDate:         rm("SelectedDate"),
		InitialConversation:  rm("InitialConversation"),
		SelectedConversation: rm("SelectedConversation"),
		ActionID:             actionId,
	}
	baa := []*slack.BlockAction{&ba}
	acb := slack.ActionCallbacks{
		BlockActions: baa,
	}
	message := slack.InteractionCallback{
		ActionCallback: acb,
		Team:           slack.Team{Domain: rm("Team")},
		User:           slack.User{Name: rm("User")},
		Channel:        slack.Channel{GroupConversation: slack.GroupConversation{Name: rm("Channel")}},
	}

	s := NewSlack(createSlackTestConfig(), mockCom, createTestDb())
	s.templateMetadata = map[string]*TemplateMetadata{
		rm("TemplateName") + ".tpl": {Name: rm("TemplateMetadata.Name")},
	}
	action := s.ConvertBlockAction(message)
	assert.Equal(t, rm("Id"), action.Id)
	assert.Equal(t, rm("TemplateName")+".tpl", action.TemplateName)
	assert.Equal(t, rm("TemplateMetadata.Name"), action.TemplateMeta.Name)
	assert.Equal(t, rm("ElasticSearchUrl"), action.Data.ElasticSearchUrl)
	assert.Equal(t, rm("ViewUrl"), action.Data.ViewUrl)
	assert.Equal(t, rm("Team"), action.Data.Team)
	assert.Equal(t, rm("Channel"), action.Data.Channel)
	assert.Equal(t, rm("User"), action.Data.User)
	assert.Equal(t, interaction, action.Data.InteractionData)
}

func TestSlack_ConvertInteractionCallback(t *testing.T) {
	rm := randMap()
	callbackId := rm("Id") + "|" + rm("TemplateName")
	env := map[string]string{
		"ElasticSearchUrl": rm("ElasticSearchUrl"),
		"ViewUrl":          rm("ViewUrl"),
	}
	mockCom := new(mocks.ChatOpsCom)
	mockCom.On("EnvironmentParams").Return(env)
	s := NewSlack(createSlackTestConfig(), mockCom, createTestDb())
	s.templateMetadata = map[string]*TemplateMetadata{
		rm("TemplateName") + ".tpl": {Name: rm("TemplateMetadata.Name")},
	}
	// dialogSubmissionCallback.Submission is string:string, so this is a shortcut.
	// since map isn't an interface we can't type assert.
	dialogSubmission := map[string]interface{}{
		rm("InteractionKey"): rm("InteractionValue"),
	}
	dialogSubmissionS := map[string]string{
		rm("InteractionKey"): rm("InteractionValue"),
	}
	// end shortcut
	icb := slack.InteractionCallback{
		CallbackID: callbackId,
		DialogSubmissionCallback: slack.DialogSubmissionCallback{
			Submission: dialogSubmissionS,
		},
		Team:    slack.Team{Domain: rm("Team")},
		User:    slack.User{Name: rm("User")},
		Channel: slack.Channel{GroupConversation: slack.GroupConversation{Name: rm("Channel")}},
	}
	action := s.ConvertInteractionCallback(icb)
	assert.Equal(t, rm("Id"), action.Id)
	assert.Equal(t, rm("TemplateName")+".tpl", action.TemplateName)
	assert.Equal(t, rm("TemplateMetadata.Name"), action.TemplateMeta.Name)
	assert.Equal(t, rm("ElasticSearchUrl"), action.Data.ElasticSearchUrl)
	assert.Equal(t, rm("ViewUrl"), action.Data.ViewUrl)
	assert.Equal(t, rm("Team"), action.Data.Team)
	assert.Equal(t, rm("Channel"), action.Data.Channel)
	assert.Equal(t, rm("User"), action.Data.User)
	assert.Equal(t, dialogSubmission, action.Data.InteractionData)
}

func TestSlack_ConvertCommandInput(t *testing.T) {
	rm := randMap()
	input := fmt.Sprintf("%s -%s %s", rm("TemplateName"), rm("Flag1"), rm("Value1"))
	args := []string{"-" + rm("Flag1"), rm("Value1")}
	inargs := util.ParseArgs(args)
	env := map[string]string{
		"ElasticSearchUrl": rm("ElasticSearchUrl"),
		"ViewUrl":          rm("ViewUrl"),
	}
	mockCom := new(mocks.ChatOpsCom)
	mockCom.On("EnvironmentParams").Return(env)
	s := NewSlack(createSlackTestConfig(), mockCom, createTestDb())
	s.templates, _ = template.New(rm("TemplateName") + ".tpl").Parse(``)
	s.templateMetadata = map[string]*TemplateMetadata{
		rm("TemplateName") + ".tpl": {Name: rm("TemplateMetadata.Name")},
	}

	action, err := s.ConvertCommandInput(rm("TeamId"), rm("TeamDomain"), rm("Channel"), rm("User"), input)
	assert.NoError(t, err)
	assert.Equal(t, rm("TemplateName")+".tpl", action.TemplateName)
	assert.Equal(t, rm("TemplateMetadata.Name"), action.TemplateMeta.Name)
	assert.Equal(t, rm("ElasticSearchUrl"), action.Data.ElasticSearchUrl)
	assert.Equal(t, rm("ViewUrl"), action.Data.ViewUrl)
	assert.Equal(t, rm("TeamDomain"), action.Data.Team)
	assert.Equal(t, rm("Channel"), action.Data.Channel)
	assert.Equal(t, rm("User"), action.Data.User)
	assert.Equal(t, inargs, action.Data.InteractionData)
}

func TestSlack_ExecuteAction(t *testing.T) {
	rm := randMap()
	env := map[string]string{
		"ElasticSearchUrl": rm("ElasticSearchUrl"),
		"ViewUrl":          rm("ViewUrl"),
	}
	mockCom := new(mocks.ChatOpsCom)
	mockCom.On("EnvironmentParams").Return(env)
	s := NewSlack(createSlackTestConfig(), mockCom, createTestDb())
	var err error
	s.templates, err = template.New(rm("TemplateName") + ".tpl").Funcs(template.FuncMap{
		"rm": rm,
	}).Parse(`
{
 "ElasticSearchUrl": "{{ .ElasticSearchUrl }}",
 "ViewUrl": "{{ .ViewUrl }}",
 "Team": "{{ .Team }}",
 "Channel" : "{{ .Channel }}",
 "User": "{{ .User }}",
 "InputText": "{{ .InputText }}",
 "InteractionData": {
    "{{ rm "DataKey1" }}": "{{ rm "DataValue1" }}"
 }
}
`)
	assert.NoError(t, err)
	s.templateMetadata = map[string]*TemplateMetadata{
		rm("TemplateName") + ".tpl": {Name: rm("TemplateMetadata.Name")},
	}

	data := map[string]interface{}{
		rm("DataKey1"): rm("DataValue1"),
	}
	action := s.NewAction(ActionInput{
		Id:              rm("Id"),
		TeamId:          rm("TeamId"),
		Team:            rm("TeamDomain"),
		Channel:         rm("Channel"),
		User:            rm("User"),
		InputText:       rm("InputText"),
		InteractionData: data,
		TemplateName:    rm("TemplateName") + ".tpl",
	})
	result, err := s.ExecuteAction(action)
	assert.NoError(t, err)

	// TODO:(smt) template metadata skipped for now.

	var td TemplateData
	err = json.Unmarshal(result.ProcessedTemplate, &td)
	assert.NoError(t, err)
	assert.Equal(t, rm("ElasticSearchUrl"), td.ElasticSearchUrl)
	assert.Equal(t, rm("ViewUrl"), td.ViewUrl)
	assert.Equal(t, rm("TeamDomain"), td.Team)
	assert.Equal(t, rm("Channel"), td.Channel)
	assert.Equal(t, rm("User"), td.User)
	assert.Equal(t, rm("InputText"), td.InputText)
	assert.Equal(t, data, td.InteractionData)
}

func TestSlack_EventHandler(t *testing.T) {
	t.SkipNow()
	// TODO:(smt) in progress

	rm := randMap()
	env := map[string]string{
		"ElasticSearchUrl": rm("ElasticSearchUrl"),
		"ViewUrl":          rm("ViewUrl"),
	}
	mockCom := new(mocks.ChatOpsCom)
	mockCom.On("EnvironmentParams").Return(env)
	s := NewSlack(createSlackTestConfig(), mockCom, createTestDb())

	body := ioutil.NopCloser(strings.NewReader("{}"))
	req, err := http.NewRequest(http.MethodGet, EventEndpoint, body)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.EventHandler)
	handler.ServeHTTP(rr, req)
}

func TestSlack_LoadTemplates(t *testing.T) {
	rand := randMap()
	cfg := createSlackTestConfig()
	cfg.TemplateDir = "testdata"
	s := NewSlack(cfg, nil, createTestDb())
	if err := s.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	tplName := fmt.Sprintf("%s.tpl", t.Name())
	fmt.Println(t.Name())
	if meta, ok := s.templateMetadata[tplName]; ok {
		assert.Equal(t, t.Name(), meta.Name)
		assert.Equal(t, "a template for testing", meta.Description)
		assert.Equal(t, true, meta.SendToKafka)
		assert.Equal(t, "feedback", meta.KafkaMessageType)
		assert.Equal(t, true, meta.Dialog)
		assert.Equal(t, true, meta.IsTerminating)
		assert.Equal(t, map[string]interface{}{"key": "value"}, meta.Extra)
	} else {
		t.FailNow()
	}
	tpl := s.templates.Lookup(tplName)
	buf := new(bytes.Buffer)
	expectedData := TemplateData{
		EnvironmentParams: EnvironmentParams{
			ElasticSearchUrl: rand("es_url"),
			ViewUrl:          rand("view_url"),
			HealthUrl:        rand("health_url"),
		},
		Team:      rand("team"),
		Channel:   rand("channel"),
		User:      rand("user"),
		InputText: rand("text"),
		Timestamp: 0,
		InteractionData: map[string]interface{}{
			"key": rand("value"),
		},
	}
	err := tpl.Execute(buf, expectedData)
	if err != nil {
		t.Fatal(err)
	}

	want, err := json.Marshal(expectedData)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(want), strings.TrimSpace(buf.String()))
}

// This test verifies all slack templates just so we don't ever inadvertently push a release with a template that doesn't parse.
func TestSlack_VerifyTemplates(t *testing.T) {
	cfg := createSlackTestConfig()
	cfg.TemplateDir = "../templates"
	s := NewSlack(cfg, nil, createTestDb())
	if err := s.LoadTemplates(); err != nil {
		t.Error("failed to load templates:", err)
	}
	for _, tpl := range s.templates.Templates() {
		fmt.Println("loaded template:", tpl.Name())
	}
}

func TestSlack_EnvParams(t *testing.T) {
	envParams := map[string]string{
		"ElasticSearchUrl": gutil.RandomString(10),
		"ViewUrl":          gutil.RandomString(10),
		"HealthUrl":        gutil.RandomString(10),
	}
	mCom := new(mocks.ChatOpsCom)
	mCom.On("EnvironmentParams").Return(envParams)
	s := NewSlack(createSlackTestConfig(), mCom, createTestDb())
	gotParams := s.EnvParams()
	assert.Equal(t, envParams["ElasticSearchUrl"], gotParams.ElasticSearchUrl)
	assert.Equal(t, envParams["ViewUrl"], gotParams.ViewUrl)
	assert.Equal(t, envParams["HealthUrl"], gotParams.HealthUrl)

	// Marshalling test helps us ensure we updated the map and struct when modifying the parameters.
	want, err := json.Marshal(envParams) // marshal map to json
	if err != nil {
		t.Fatal(err)
	}
	var got EnvironmentParams
	if err := json.Unmarshal(want, &got); err != nil { // unmarshal the marshalled map
		t.Fatal(err)
	}
	assert.Equal(t, gotParams, got) // verify map matches the env params
}

func TestTemplateData_FeedbackMessage(t *testing.T) {
	want := FeedbackMessage{
		User:   "seth",
		Value:  1.2,
		Label:  4.0,
		AtsuId: "atsuatsuatsuatasu",
		Etype:  "asdf123",
	}

	td := TemplateData{
		EnvironmentParams: EnvironmentParams{},
		Team:              "",
		Channel:           "",
		User:              want.User,
		InputText:         "",
		Timestamp:         0,
		InteractionData: map[string]interface{}{
			"value": fmt.Sprintf(`{"value":%f,"label":%f,"atsu_id":"%s","etype":"%s"}`,
				want.Value, want.Label, want.AtsuId, want.Etype),
		},
	}
	got := td.FeedbackMessage()
	assert.Equal(t, want, got)
}

func TestSlack_KafkaSend(t *testing.T) {
	tests := []struct {
		name  string
		typ   KafkaMessageType
		topic string
	}{
		{"default", "", "slack"},
		{"feedback", Feedback, "feedtop"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := TemplateData{
				Team:      "",
				Channel:   "",
				User:      "",
				InputText: "",
				Timestamp: 0,
				InteractionData: map[string]interface{}{
					"value": `{"atsu_id":"asdfsdf"}`,
				},
			}
			var obj interface{}
			switch test.typ {
			case Feedback:
				obj = data.FeedbackMessage()
			default:
				obj = data
			}
			b, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}

			mCom := new(mocks.ChatOpsCom)
			mCom.On("KafkaProduce", mock.MatchedBy(func(topic string) bool {
				return topic == test.topic
			}), mock.MatchedBy(func(msg string) bool {
				return string(b) == msg
			}))
			cfg := createSlackTestConfig()
			cfg.FeedbackTopic = test.topic
			s := NewSlack(cfg, mCom, createTestDb())
			s.KafkaSend(test.typ, data)
		})
	}
}

func createSlackTestConfig() SlackConfig {
	return SlackConfig{
		Token:             "",
		VerificationToken: "",
		SecretSigningKey:  "",
		InWebHook:         "",
		FeedbackTopic:     "",
		ClientId:          "",
		ClientSecret:      "",
		TemplateDir:       "",
	}
}

type TestDb struct {
}

func createTestDb() *TestDb {
	return &TestDb{}
}

func (t TestDb) Init() error {
	return nil
}

func (t TestDb) InsertSlackBot(teamId, botToken, webHookUrl string) error {
	return nil
}

func (t TestDb) GetSlackBot(teamId string) (string, string, error) {
	return "", "", nil
}

func (t TestDb) GetAllSlackBots() ([]db.SlackBot, error) {
	return nil, nil
}
