package bot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/atsu/chatops/db"
	"github.com/atsu/chatops/interfaces"
	"github.com/atsu/chatops/relay"
	"github.com/atsu/chatops/util"
	"github.com/atsu/goat/health"
	"github.com/gorilla/mux"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/zserge/metric"
)

const (
	botUserName              = "atsu"
	EventEndpoint            = "/slack/event"
	InterctEndpoint          = "/slack/interact"
	SlashEndpoint            = "/slack/slash"
	LoadActionsEndpoint      = "/slack/load-actions"
	AtsuEventEndpoint        = "/slack/atsu-event"
	SlackAuthorizeEndpoint   = "/slack/authorize"
	SlackCallbackEndpoint    = "/slack/callback"
	SlackOnDemandTplEndpoint = "/slack/on-demand-template"
	FreeformTemplate         = "_freeform.tpl"
	FreeformPassword         = "YXRzdS10by10aGUtbW9vbiEhIQ==" // TODO XXX hard-coded for now
	SlackAuthorizeUrl        = "https://slack.com/oauth/v2/authorize"
	SlackAccessUrl           = "https://slack.com/api/oauth.v2.access"
)

type Slack struct {
	token             string
	verificationToken string
	secretSigningKey  string
	inWebHook         string
	feedbackTopic     string
	clientId          string
	clientSecret      string
	authRedirectUrl   string

	// map of id to slack client
	//workspaceApis map[string]*slack.Client
	workspaceApis sync.Map

	database          db.Database
	com               interfaces.ChatOpsCom
	templateDirectory string
	templates         *template.Template
	templateMetadata  map[string]*TemplateMetadata
	onDemandTemplates *template.Template
	onDemandMetadata  map[string]*TemplateMetadata
	odtLock           sync.Mutex

	requestResponseTimeSecs metric.Metric
	slashCounter            metric.Metric
	eventCounter            metric.Metric
	interactCounter         metric.Metric
	loadActionsCounter      metric.Metric
	atsuEventsCounter       metric.Metric
	templateErrorsCounter   metric.Metric
	errorsLastHour          metric.Metric
	errorCount              int64
	errorTimes              []int64
	errorsRecent            []string
	errLock                 sync.Mutex

	//api         *slack.Client
	doneCh      chan int
	resultQueue chan *ActionResult
	debug       bool
}

type SlackConfig struct {
	AppId             string
	Token             string
	VerificationToken string
	SecretSigningKey  string
	InWebHook         string
	FeedbackTopic     string
	ClientId          string
	ClientSecret      string
	AuthRedirectUrl   string

	TemplateDir string
}

func (cfg SlackConfig) Validate() error {
	// TODO: validate the config
	return nil
}

func NewSlack(cfg SlackConfig, com interfaces.ChatOpsCom, database db.Database) *Slack {
	return &Slack{
		token:             cfg.Token,
		verificationToken: cfg.VerificationToken,
		secretSigningKey:  cfg.SecretSigningKey,
		inWebHook:         cfg.InWebHook,
		feedbackTopic:     cfg.FeedbackTopic,
		clientId:          cfg.ClientId,
		clientSecret:      cfg.ClientSecret,
		authRedirectUrl:   cfg.AuthRedirectUrl,
		templateDirectory: path.Join(cfg.TemplateDir, "slack"),

		//workspaceApis: make(map[string]*slack.Client),

		requestResponseTimeSecs: metric.NewHistogram("1h1h"), // 1 hour history, 1 hour precision
		slashCounter:            metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		eventCounter:            metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		interactCounter:         metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		loadActionsCounter:      metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		atsuEventsCounter:       metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		templateErrorsCounter:   metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		errorsLastHour:          metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		onDemandMetadata:        make(map[string]*TemplateMetadata),
		onDemandTemplates:       template.New(""), // dummy so we don't ever hit a nil pointer.

		database:     database,
		com:          com,
		errorTimes:   make([]int64, 0, 100),
		errorsRecent: make([]string, 0, 10),
		doneCh:       make(chan int),
		resultQueue:  make(chan *ActionResult, 10),
	}
}

func (s *Slack) SetDebug(b bool) {
	s.debug = b
}

type SlackStatus struct {
	Health                health.State `json:"health"`
	ResponseTimeSecs      interface{}
	SlashCounter          interface{}
	EventCounter          interface{}
	InteractCounter       interface{}
	LoadActionCounter     interface{}
	AtsuEventCounter      interface{}
	ErrorsCounter         interface{}
	TemplateErrorsCounter interface{}
	ErrorTimes            []int64
	ErrorsRecent          []string
	Events                int64 `json:"events"`
	Errors                int64 `json:"errors"`
}

func (s *Slack) LoadTemplates() error {
	templates, templateMetadata, err := ReadTemplates(s.templateDirectory)
	if err == nil {
		s.templates = templates
		s.templateMetadata = templateMetadata
	}

	return err
}

func (s *Slack) Status() SlackStatus {
	h := health.Green
	return SlackStatus{
		Health:                h,
		ResponseTimeSecs:      s.requestResponseTimeSecs,
		SlashCounter:          s.slashCounter,
		EventCounter:          s.eventCounter,
		InteractCounter:       s.interactCounter,
		LoadActionCounter:     s.loadActionsCounter,
		AtsuEventCounter:      s.atsuEventsCounter,
		ErrorsCounter:         s.errorsLastHour,
		TemplateErrorsCounter: s.templateErrorsCounter,
		ErrorTimes:            s.errorTimes,
		ErrorsRecent:          s.errorsRecent,
		Errors:                s.errorCount,
	}
}

type SlackInstance struct {
	TeamId     string
	WebHookUrl string
	BotToken   string
	client     *slack.Client
}

func (s *Slack) Start(router *mux.Router, r *relay.Relay) error {
	if r.Mode != relay.PassThrough {
		if err := s.LoadTemplates(); err != nil {
			return err
		}

		// TODO: this should really be on-demand loading
		bots, err := s.database.GetAllSlackBots()
		if err != nil {
			return err
		}
		for _, bot := range bots {
			inst := SlackInstance{
				TeamId:     bot.TeamId,
				BotToken:   bot.BotToken,
				WebHookUrl: bot.WebHookUrl,
				client:     slack.New(bot.BotToken),
			}
			ti, err := inst.client.GetTeamInfo()
			if err != nil {
				log.Printf("error loading slack client for team:%s token:%s - %s", bot.TeamId, bot.BotToken, err)
			} else {
				log.Printf("slack for teamid:%s (%s) on \"%s.slack.com\" is active\n", ti.ID, ti.Name, ti.Domain)
				s.workspaceApis.Store(bot.TeamId, inst)
			}
		}
		s.startResponseProcessor()
	}

	// For relay mode, we want to relay the slack events...
	if r.Mode == relay.PassThrough {
		// Passthrough Mode only relays requests
		router.HandleFunc(EventEndpoint, r.RelayHandler)
		router.HandleFunc(InterctEndpoint, r.RelayHandler)
		router.HandleFunc(SlashEndpoint, r.RelayHandler)
		router.HandleFunc(LoadActionsEndpoint, r.RelayHandler)
		router.HandleFunc(AtsuEventEndpoint, r.RelayHandler)
		router.HandleFunc(SlackOnDemandTplEndpoint, r.RelayHandler)
		router.HandleFunc(SlackAuthorizeEndpoint, r.RelayHandler)
		router.HandleFunc(SlackCallbackEndpoint, r.RelayHandler)
	} else {
		if r.Mode == relay.Handler {
			// Handler Mode handles relayed requests.
			r.HandleFunc(EventEndpoint, s.EventHandler)
			r.HandleFunc(InterctEndpoint, s.DelayedInteractionHandler)
			r.HandleFunc(SlashEndpoint, s.SlashHandler)
			r.HandleFunc(LoadActionsEndpoint, s.ImmediateInteractionHandler)
			r.HandleFunc(AtsuEventEndpoint, s.AtsuEventHandler)
			r.HandleFunc(SlackOnDemandTplEndpoint, s.OnDemandTemplateHandler)
			r.HandleFunc(SlackAuthorizeEndpoint, s.AuthorizeHandler)
			r.HandleFunc(SlackCallbackEndpoint, s.CallbackHandler)
		}

		// Any mode that is not Passthrough will handle local requests
		router.HandleFunc(EventEndpoint, s.EventHandler)
		router.HandleFunc(InterctEndpoint, s.DelayedInteractionHandler)
		router.HandleFunc(SlashEndpoint, s.SlashHandler)
		router.HandleFunc(LoadActionsEndpoint, s.ImmediateInteractionHandler)
		router.HandleFunc(AtsuEventEndpoint, s.AtsuEventHandler)
		router.HandleFunc(SlackOnDemandTplEndpoint, s.OnDemandTemplateHandler)
		router.HandleFunc(SlackAuthorizeEndpoint, s.AuthorizeHandler)
		router.HandleFunc(SlackCallbackEndpoint, s.CallbackHandler)
	}

	return nil
}

func (s *Slack) startResponseProcessor() {
	go func() {
		for {
			select {
			case <-s.doneCh:
				return
			case res := <-s.resultQueue:
				s.SendResultResponse(res)
			}
		}
	}()
}

func (s *Slack) Stop() {
	if s.doneCh != nil {
		close(s.doneCh)
	}
}

func (s *Slack) httpError(r *http.Request, w http.ResponseWriter, status int, resp string, e error) {
	if e != nil {
		log.Printf("%q request from %q against %q failed: %v\n - responding with: %s", r.Method, r.RemoteAddr, r.RequestURI, e, resp)
	}
	if resp != "" {
		http.Error(w, resp, status)
	} else {
		w.WriteHeader(status)
	}
	s.recordError(e)
}

func (s *Slack) recordError(err error) {
	s.errLock.Lock()
	defer s.errLock.Unlock()
	s.errorsLastHour.Add(1)
	if len(s.errorTimes) > 100 {
		s.errorTimes = s.errorTimes[1:100]
	}
	s.errorTimes = append(s.errorTimes, time.Now().Unix())
	if len(s.errorsRecent) > 10 {
		s.errorsRecent = s.errorsRecent[1:10]
	}
	if err != nil {
		s.errorsRecent = append(s.errorsRecent, err.Error())
	}
	s.errorCount++
}

/* OnDemandTemplateHandler is a handler that will attempt to load a template from the
post body payload.
ex:
*/
// {{/* Template Info
// ---
// name: od-test
// description: an example on demand template
// ---
// */}}
//{"blocks": [{
//            "type": "section",
//            "text": {
//                "type": "mrkdwn",
//                "text": "{{ .InteractionData.text }}"
//            }}]
//}
//
// see the AtsuEventHandler for information about on demand templates.
func (s *Slack) OnDemandTemplateHandler(w http.ResponseWriter, r *http.Request) {
	s.atsuEventsCounter.Add(1)
	start := time.Now()
	defer func() { s.requestResponseTimeSecs.Add(time.Since(start).Seconds()) }()
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}

	s.odtLock.Lock()
	defer s.odtLock.Unlock()
	tpl, meta, err := ReadOnDemandTemplate(r.Body, s.onDemandTemplates)
	if err != nil {
		s.recordError(err)
		http.Error(w, "failed to parse body", http.StatusBadRequest)
		return
	}

	log.Println("storing on demand template:", tpl.Name())
	s.onDemandTemplates = tpl
	s.onDemandMetadata[tpl.Name()] = meta

	w.WriteHeader(http.StatusAccepted)
}

func (s *Slack) AuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	//r.URL.Query().Set("redirect_uri", "http://localhost")
	//r.URL.Query().Set("scope", "incoming-webhook")
	//r.URL.Query().Set("client_id", s.clientId)
	//r.URL.Query().Set("state", "")
	scopes := "app_mentions:read,incoming-webhook,commands,team:read"
	u, _ := url.Parse(SlackAuthorizeUrl)
	u.RawQuery = fmt.Sprintf("scope=%s&client_id=%s", scopes, s.clientId)
	http.Redirect(w, r, u.String(), http.StatusFound)
}

type Identity struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type SlackWebHook struct {
	Channel          string `json:"channel"`
	ChannelId        string `json:"channel_id"`
	ConfigurationUrl string `json:"configuration_url"`
	Url              string `json:"url"`
}

type SlackAuthResponse struct {
	Ok              bool         `json:"ok"`
	AccessToken     string       `json:"access_token"`
	TokenType       string       `json:"token_type"`
	Scope           string       `json:"scope"`
	BotUserId       string       `json:"bot_user_id"`
	AppId           string       `json:"app_id"`
	Team            Identity     `json:"team"`
	Enterprise      Identity     `json:"enterprise"`
	IncomingWebHook SlackWebHook `json:"incoming_webhook"`
	//AuthedUser string // TODO:(smt) do we care about this?
}

func (s *SlackAuthResponse) String() string {
	if b, err := json.Marshal(s); err != nil {
		return fmt.Sprintf(`{"error": "%s" }`, err.Error())
	} else {
		return string(b)
	}
}

func (s *Slack) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	//state := r.URL.Query().Get("state")

	form := url.Values{
		"code":          {code},
		"client_id":     {s.clientId},
		"client_secret": {s.clientSecret},
	}
	res, err := http.PostForm(SlackAccessUrl, form)
	if err != nil {
		log.Println(err)
	} else {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println(string(body))
		var auth SlackAuthResponse
		if err := json.Unmarshal(body, &auth); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Println(auth)
		inst := SlackInstance{
			TeamId:     auth.Team.Id,
			WebHookUrl: auth.IncomingWebHook.Url,
			BotToken:   auth.AccessToken,
			client:     slack.New(auth.AccessToken),
		}
		s.workspaceApis.Store(auth.Team.Id, inst)
		if err := s.database.InsertSlackBot(auth.Team.Id, auth.AccessToken, auth.IncomingWebHook.Url); err != nil {
			log.Println(err)
		}
	}
	http.Redirect(w, r, s.authRedirectUrl, http.StatusFound)
}

// EventHandler handles events from the slack events api (https://api.slack.com/events-api)
// this is for subscribed events such as mentions
func (s *Slack) EventHandler(w http.ResponseWriter, r *http.Request) {
	s.eventCounter.Add(1)
	start := time.Now()
	defer func() { s.requestResponseTimeSecs.Add(time.Since(start).Seconds()) }()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "invalid body", err)
		return
	}
	if s.debug {
		log.Println(string(body))
	}

	eventsAPIEvent, e := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if e != nil {
		s.httpError(r, w, http.StatusInternalServerError, "failed to parse", nil)
		return
	}

	switch eventsAPIEvent.Type {
	case slackevents.AppRateLimited:
		log.Println("rate limit event received")
	case slackevents.URLVerification:
		var cr slackevents.ChallengeResponse
		err := json.Unmarshal(body, &cr)
		if err != nil {
			s.httpError(r, w, http.StatusInternalServerError, "unmarshal failed", nil)
			return
		}
		w.Header().Set("Content-Type", "text")
		if _, err := w.Write([]byte(cr.Challenge)); err != nil {
			log.Println(err)
		}

	case slackevents.CallbackEvent:
		if !s.VerifyRequest(r.Header, body) {
			s.httpError(r, w, http.StatusUnauthorized, "validation failed", fmt.Errorf("[ERROR] Failed validating signed secret"))
			return
		}
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			// follow up processing asynchronously to allow the request to close
			go func() {
				if result, err := s.processAppMentionEvent(eventsAPIEvent.TeamID, "", ev); err != nil {
					log.Println("app mention event failed:", err)
					s.SendErrorResponse(err, Channel, eventsAPIEvent.TeamID, ev.Channel, "")
				} else {
					s.queueActionResult(result)
				}
			}()
		}
	}
}

// DelayedInteractionHandler handles interactivity events from slack (https://api.slack.com/messaging/interactivity#components)
func (s *Slack) DelayedInteractionHandler(w http.ResponseWriter, r *http.Request) {
	s.interactCounter.Add(1)
	start := time.Now()
	defer func() { s.requestResponseTimeSecs.Add(time.Since(start).Seconds()) }()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "", err)
		return
	}

	err, status := s.validateInteraction(r.Header, r.Method, body)
	if err != nil {
		s.httpError(r, w, status, fmt.Sprintf("validation failed from: %s", r.RemoteAddr), err)
		return
	}

	payload := bytes.TrimPrefix(body, []byte("payload="))
	jsonStr, err := url.QueryUnescape(string(payload))
	if err != nil {
		s.httpError(r, w, http.StatusBadRequest, "body payload must be escaped json", err)
		return
	}
	message, err := ParseInteractionCallback([]byte(jsonStr))
	if err != nil {
		s.httpError(r, w, http.StatusBadRequest, "invalid body", err)
		return
	}

	// if the request did not require an immediate response,
	// Then we need to respond within 3000ms with content,
	// or just 200 and we can use the ResponseURL later
	w.WriteHeader(http.StatusOK)

	// follow up processing asynchronously to allow the request to close
	go func() {
		if result, err := s.processInteractionCallback(message); err != nil {
			log.Println("interaction failed:", err)
			s.SendErrorResponse(err, Direct, message.Team.ID, message.Channel.Name, message.ResponseURL)
		} else {
			s.queueActionResult(result)
		}
	}()
}

// SlashHandler handles slack slash command events (https://api.slack.com/slash-commands)
func (s *Slack) SlashHandler(w http.ResponseWriter, r *http.Request) {
	s.slashCounter.Add(1)
	start := time.Now()
	defer func() { s.requestResponseTimeSecs.Add(time.Since(start).Seconds()) }()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "invalid body", err)
		return
	}
	if !s.VerifyRequest(r.Header, body) {
		s.httpError(r, w, http.StatusUnauthorized, "not authorized", nil)
		return
	}

	// set the body back for slash command parsing
	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	sc, err := slack.SlashCommandParse(r)
	if err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "failed to parse command", err)
		return
	}

	// Disable atsu check
	//if sc.Command != "/atsu" {
	//	s.httpError(r, w, http.StatusBadRequest, "unknown command", nil)
	//	return
	//}

	// Here we need to respond within 3000ms with content,
	// or just 200 and we can use the ResponseURL later
	w.WriteHeader(http.StatusOK)

	// follow up processing asynchronously to allow the request to close
	go func() {
		if result, err := s.processSlashCommand(sc); err != nil {
			s.SendErrorResponse(err, Direct, sc.TeamID, sc.ChannelName, sc.ResponseURL)
		} else {
			s.queueActionResult(result)
		}
	}()
}

// ImmediateInteractionHandler handles interactivity events from slack (https://api.slack.com/messaging/interactivity#components)
func (s *Slack) ImmediateInteractionHandler(w http.ResponseWriter, r *http.Request) {
	s.loadActionsCounter.Add(1)
	start := time.Now()
	defer func() { s.requestResponseTimeSecs.Add(time.Since(start).Seconds()) }()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "invalid body", err)
		return
	}
	err, status := s.validateInteraction(r.Header, r.Method, body)
	if err != nil {
		s.httpError(r, w, status, "validation failed", err)
		return
	}
	payload, err := util.DecodePayloadBody(body)
	if err != nil {
		s.httpError(r, w, http.StatusBadRequest, "decode failure", err)
		return
	}
	message, err := ParseInteractionCallback(payload)
	if err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "parse interaction failure", err)
		return
	}
	// handle immediate responses
	if result, err := s.processInteractionCallback(message); err != nil {
		s.httpError(r, w, http.StatusInternalServerError, "interaction failure", err)
	} else {
		if _, err := w.Write(result.ProcessedTemplate); err != nil {
			log.Println(err)
		}
	}
}

// AtsuEventHandler handles incoming requests from atsu
// this is a mechanism to translate an incoming request to a slack event.
// On demand templates can be run if the 'od' query parameter is truthy.
func (s *Slack) AtsuEventHandler(w http.ResponseWriter, r *http.Request) {
	// TODO:(smt) this needs some kind of authentication
	s.atsuEventsCounter.Add(1)
	start := time.Now()
	defer func() { s.requestResponseTimeSecs.Add(time.Since(start).Seconds()) }()

	odstr := r.URL.Query().Get("od")
	od, _ := strconv.ParseBool(odstr) // error is falsey

	if r.Method != http.MethodPost {
		s.httpError(r, w, http.StatusMethodNotAllowed, "not allowed", nil)
		return
	}
	teamId := r.URL.Query().Get("teamId")
	tpl := r.URL.Query().Get("tpl") // specify template
	pw := r.URL.Query().Get("pw")
	freeformEnabled := pw == FreeformPassword && tpl == FreeformTemplate

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.httpError(r, w, http.StatusBadRequest, "bad request", err)
		return
	}

	var data map[string]interface{}
	if !freeformEnabled {
		if err := json.Unmarshal(body, &data); err != nil {
			s.httpError(r, w, http.StatusBadRequest, err.Error(), err)
			return
		}
	}
	action := &Action{
		OnDemand:     od,
		TeamId:       teamId,
		ResponseType: WebHook,
		TemplateName: tpl,
		Data: TemplateData{
			EnvironmentParams: s.EnvParams(),
			InteractionData:   data,
			Timestamp:         time.Now().Unix(),
		},
	}

	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Println(err)
	}

	// follow up processing asynchronously to allow the request to close
	go func() {
		if result, err := s.ExecuteAction(action); err != nil {
			log.Printf("failed processing atsu event action: %s - %s", action, err)
		} else {
			if freeformEnabled {
				result.ProcessedTemplate = body
			}
			s.queueActionResult(result)
		}
	}()
}

type EnvironmentParams struct {
	ElasticSearchUrl string
	ViewUrl          string
	HealthUrl        string
}

func (s *Slack) EnvParams() EnvironmentParams {
	params := EnvironmentParams{}
	m := s.com.EnvironmentParams()
	if esu, ok := m["ElasticSearchUrl"]; ok {
		params.ElasticSearchUrl = esu
	}
	if vu, ok := m["ViewUrl"]; ok {
		params.ViewUrl = vu
	}
	if hu, ok := m["HealthUrl"]; ok {
		params.HealthUrl = hu
	}
	return params
}

func (s *Slack) processAppMentionEvent(teamId, teamDomain string, ev *slackevents.AppMentionEvent) (*ActionResult, error) {
	// Currently overriding to send any mentions to kafka
	// TODO:(smt) lookup user/chan/team ids
	action := s.NewAction(ActionInput{
		Id:              "",
		TeamId:          teamId,
		Team:            teamDomain,
		Channel:         ev.Channel,
		User:            ev.User,
		InputText:       ev.Text,
		InteractionData: make(map[string]interface{}),
		TemplateName:    "_kafka.tpl",
	})
	action.SetResponse(Channel, "", ev.Channel, "")
	return s.ExecuteAction(action)
}

func (s *Slack) processSlashCommand(sc slack.SlashCommand) (*ActionResult, error) {
	sa, err := s.ConvertCommandInput(sc.TeamID, sc.TeamDomain, sc.ChannelName, sc.UserName, sc.Text)
	if err != nil {
		return nil, fmt.Errorf("failed converting command input: %v", err)
	}
	sa.SetResponse(Direct, sc.ResponseURL, sc.ChannelName, sc.TriggerID)
	result, err := s.ExecuteAction(sa)
	if err != nil {
		return nil, fmt.Errorf("processing slash command failed: %v", err)
	}
	return result, nil
}

func (s *Slack) processInteractionCallback(message slack.InteractionCallback) (*ActionResult, error) {
	if s.debug {
		log.Println("Interaction Type:", message.Type)
	}
	var action *Action
	var rt ResponseType
	switch message.Type {
	case slack.InteractionTypeBlockActions:
		rt = Direct
		action = s.ConvertBlockAction(message)
	case slack.InteractionTypeDialogSuggestion:
		fallthrough
	case slack.InteractionTypeDialogCancellation:
		rt = Direct
		action = s.ConvertInteractionCallback(message)
	case slack.InteractionTypeDialogSubmission:
		rt = None
		action = s.ConvertInteractionCallback(message)
	default:
		log.Println("unknown interaction type:", message.Type)
	}
	if s.debug {
		log.Printf("processingInteractionCallback: [%s] Action: %s", message.Type, action)
	}
	if action != nil {
		action.SetResponse(rt, message.ResponseURL, message.Channel.Name, message.TriggerID)
		return s.ExecuteAction(action)
	}
	return nil, errors.New("invalid action")
}

func (s *Slack) queueActionResult(result *ActionResult) {
	if result != nil {
		s.resultQueue <- result
	}
}

// VerifyRequest https://api.slack.com/docs/verifying-requests-from-slack
func (s *Slack) VerifyRequest(h http.Header, body []byte) bool {
	timestamp := h.Get("X-Slack-Request-Timestamp")
	str := fmt.Sprintf("%s:%s:%s", "v0", timestamp, string(body))
	hash := util.ComputeSha256HMAC([]byte(str), []byte(s.secretSigningKey))
	return fmt.Sprintf("v0=%s", hash) == h.Get("X-Slack-Signature")
}

// validate the interaction, and returns an error if validation fails.
// if an error is returned the second result is the http status code for response
func (s *Slack) validateInteraction(header http.Header, method string, body []byte) (error, int) {
	if method != http.MethodPost {
		return fmt.Errorf("[ERROR] Invalid method: %s", method), http.StatusMethodNotAllowed
	}
	if !s.VerifyRequest(header, body) {
		return fmt.Errorf("[ERROR] Failed validating signed secret"), http.StatusUnauthorized
	}

	// Deprecated verification
	// if s.verificationToken != message.Token {
	// 	return fmt.Errorf("[ERROR] Invalid token: %s", message.Token), http.StatusUnauthorized, nil
	// }

	return nil, 0
}

// SendToChannel sends the provided options list to the provided slack channel
func (s *Slack) SendToChannel(team string, channel string, options ...slack.MsgOption) {
	post := slack.NewPostMessageParameters()
	post.Username = botUserName
	post.AsUser = true
	post.Parse = "full"

	options = append(options, slack.MsgOptionPostMessageParameters(post))
	i, ok := s.workspaceApis.Load(team)
	if !ok {
		log.Printf("api for [%s] not found, aborting sending to channel [%s]\n", team, channel)
		return
	}
	instance, ok := i.(SlackInstance)
	if !ok {
		log.Printf("unexpected type %T not *slack.Client\n", i)
		return
	}
	if _, _, err := instance.client.PostMessage(channel, options...); err != nil {
		log.Println("failed sending to channel:", err)
	}
}

// SendErrorResponse is for sending a direct response through slack to the user (via response url) or channel
// this effectively populates the default error template and posts it back to slack for the
// 'sorry an error happened' type failure.
func (s *Slack) SendErrorResponse(err error, rt ResponseType, teamId, channel, responseUrl string) {
	result := &ActionResult{
		Action:            &Action{ResponseType: rt},
		TeamId:            teamId,
		ResponseType:      Direct,
		Channel:           channel,
		ResponseUrl:       responseUrl,
		Error:             err,
		ProcessedTemplate: s.processErrorTemplate(err),
	}
	s.SendResultResponse(result)
}

func (s *Slack) processErrorTemplate(err error) []byte {
	tpl := s.templates.Lookup("error.tpl")
	if tpl == nil {
		log.Println("error template missing, please create 'error.tpl' in the 'slack' template directory")
		log.Println(err)
		return nil
	}
	buf := new(bytes.Buffer)
	// TODO:(smt) do we want to respond with errors, *note* attempting json.Marshal(err.Error()) will only escape inner quotes, but the resulting string will be quoted...
	if e := tpl.ExecuteTemplate(buf, tpl.Name(), map[string]string{
		"Error": "an error occurred", // err.Error(),
	}); e != nil {
		log.Println("failed to process error template:", e)
	}
	return buf.Bytes()
}

// Action is an encapsulation of a slack template execution.
// We run the template name against the template data.
type Action struct {
	OnDemand     bool // should this action use an on-demand template
	ResponseType ResponseType
	ResponseUrl  string
	Channel      string
	TriggerId    string
	Id           string
	TeamId       string
	TemplateName string
	TemplateMeta TemplateMetadata
	Data         TemplateData
}

func (a *Action) SetResponse(rt ResponseType, responseUrl, channel, triggerId string) {
	a.ResponseType = rt
	a.Channel = channel
	a.ResponseUrl = responseUrl
	a.TriggerId = triggerId
}

func (a *Action) String() string {
	b, err := json.Marshal(a)
	if err != nil {
		return fmt.Sprintf("error marshalling action: %s", err)
	}
	return string(b)
}

type ResponseType string

var None = ResponseType("none")
var Direct = ResponseType("direct")
var Dialog = ResponseType("dialog")
var Channel = ResponseType("channel")
var WebHook = ResponseType("webhook")

type ActionResult struct {
	TeamId            string
	Error             error
	Action            *Action
	ResponseUrl       string
	Channel           string
	TriggerId         string
	ResponseType      ResponseType
	SendToKafka       bool
	KafkaMessageType  KafkaMessageType
	Data              TemplateData
	ProcessedTemplate []byte
}

func (ar ActionResult) String() string {
	dup := ar
	dup.ProcessedTemplate = nil
	b, err := json.Marshal(dup)
	if err != nil {
		return fmt.Sprintf("error marshalling action: %s", err)
	}
	return string(b)
}

// KafkaMessageType dictates the how messages should be sent to kafka
// it can change the message format as well as which topic sent to.
type KafkaMessageType string

const (
	// Feedback is the kafka message type for responding to alerts, see `_alert_response.tpl`
	Feedback = KafkaMessageType("feedback")
)

// FeedbackMessage corresponds to the KafkaMessageType `Feedback`, this struct dictates the json format.
type FeedbackMessage struct {
	User   string  `json:"user"`
	Label  float64 `json:"label"`
	Value  float64 `json:"value,omitempty"`
	AtsuId string  `json:"atsu_id,omitempty"`
	Etype  string  `json:"etype,omitempty"`
}

// KafkaSend will produce a message to kafka based on the message type and template data.
func (s *Slack) KafkaSend(mt KafkaMessageType, data TemplateData) {
	topic := "slack"
	var obj interface{}
	switch mt {
	case Feedback:
		topic = s.feedbackTopic
		obj = data.FeedbackMessage()
	default:
		obj = data
	}
	if b, err := json.Marshal(obj); err != nil {
		log.Printf("failed to marshal [%T] object: %v - %v", obj, obj, err)
	} else {
		s.com.KafkaProduce(topic, string(b))
	}
}

// SendResultResponse evaluates the ActionResult and sends the appropriate response payload back to slack
func (s *Slack) SendResultResponse(result *ActionResult) {
	if s.debug {
		log.Println("ActionResult:", result.String())
	}
	if result.SendToKafka {
		s.KafkaSend(result.KafkaMessageType, result.Data)
	}
	message := "->"
	if result.Error != nil {
		message = fmt.Sprintf("%s Error: %v", message, result.Error)

	}
	if result.Action != nil {
		message = fmt.Sprintf("%s Action: %s", message, result.Action)
	}
	var err error
	var b []byte
	var code int
	switch result.ResponseType {
	case None:
	case Channel:
		var msg slack.Message
		msg, err = ParseMessage(result.ProcessedTemplate)
		if err == nil {
			var opts slack.MsgOption
			if msg.Blocks.BlockSet != nil {
				message = fmt.Sprint(message, " [block set] ")
				opts = slack.MsgOptionBlocks(msg.Blocks.BlockSet...)
			} else {
				message = fmt.Sprint(message, " [not block set] ")
				opts = slack.MsgOptionText(msg.Text, false)
			}
			s.SendToChannel(result.TeamId, result.Channel, opts)
		}
	case Dialog:
		var d slack.Dialog
		d, err = ParseDialog(result.ProcessedTemplate)
		if err != nil {
			break
		}
		message = fmt.Sprint(message, " [open dialog] ")
		i, ok := s.workspaceApis.Load(result.TeamId)
		if !ok {
			err = fmt.Errorf("api for [%s] not found, aborting [open dialog]\n", result.TeamId)
			break
		}
		instance, ok := i.(SlackInstance)
		if !ok {
			err = fmt.Errorf("unexpected type %T not *slack.Client\n", i)
			break
		}
		err = instance.client.OpenDialog(result.TriggerId, d)

	case Direct:
		code, b, err = util.SendResponseURL(result.ResponseUrl, result.ProcessedTemplate)
	case WebHook:
		i, ok := s.workspaceApis.Load(result.TeamId)
		if !ok {
			err = fmt.Errorf("api for [%s] not found, aborting [WebHook response]\n", result.TeamId)
			break
		}
		instance, ok := i.(SlackInstance)
		if !ok {
			err = fmt.Errorf("unexpected type %T not *slack.Client\n", i)
			break
		}
		code, b, err = util.SendResponseURL(instance.WebHookUrl, result.ProcessedTemplate)
	default:
		err = errors.New("unknown response type")
	}
	if err != nil {
		message = fmt.Sprintf("%s - err: %v", message, err)
	}
	if err != nil || result.Error != nil {
		log.Printf("[ERROR] Status: %d - %s\n", code, message)
		s.recordError(err)
	} else {
		log.Printf("[SUCCESS] %s Status: %d Body: %s\n", message, code, string(b))
	}
}

// ConvertBlockAction translates a block action into an Action
func (s *Slack) ConvertBlockAction(message slack.InteractionCallback) *Action {
	var ba *slack.BlockAction
	for _, b := range message.ActionCallback.BlockActions {
		ba = b // This should only ever be 1?
	}
	if ba == nil {
		return nil
	}
	var id, templateName string
	spl := strings.Split(ba.ActionID, "|")
	if len(spl) > 0 {
		id = spl[0]
	}
	if len(spl) > 1 {
		templateName = fmt.Sprintf("%s.tpl", spl[1])
	}
	iadata := map[string]interface{}{
		"text":           ba.Text.Text,
		"value":          ba.Value,
		"i-channel":      ba.InitialChannel,
		"channel":        ba.SelectedChannel,
		"i-option":       ba.InitialOption.Value,
		"option":         ba.SelectedOption.Value,
		"i-user":         ba.InitialUser,
		"user":           ba.SelectedUser,
		"i-date":         ba.InitialDate,
		"date":           ba.SelectedDate,
		"i-conversation": ba.InitialConversation,
		"conversation":   ba.SelectedConversation,
	}
	return s.NewAction(ActionInput{
		Id:              id,
		TeamId:          message.Team.ID,
		Team:            message.Team.Domain,
		Channel:         message.Channel.Name,
		User:            message.User.Name,
		InputText:       "",
		InteractionData: iadata,
		TemplateName:    templateName,
	})
}

func (s *Slack) ConvertInteractionCallback(sa slack.InteractionCallback) *Action {
	var id, templateName string
	spl := strings.Split(sa.CallbackID, "|")
	if len(spl) > 0 {
		id = spl[0]
	}
	if len(spl) > 1 {
		templateName = fmt.Sprintf("%s.tpl", spl[1])
	}
	indata := make(map[string]interface{})
	for k, v := range sa.DialogSubmissionCallback.Submission {
		indata[k] = v
	}
	return s.NewAction(ActionInput{
		Id:              id,
		TeamId:          sa.Team.ID,
		Team:            sa.Team.Domain,
		Channel:         sa.Channel.Name,
		User:            sa.User.Name,
		InputText:       "",
		InteractionData: indata,
		TemplateName:    templateName,
	})
}

type ActionInput struct {
	OnDemand        bool
	Id              string
	TeamId          string
	Team            string
	Channel         string
	User            string
	InputText       string
	InteractionData map[string]interface{}
	TemplateName    string
}

func (ai *ActionInput) Validate() error {
	switch {
	case ai.InteractionData == nil:
		return fmt.Errorf("interaction data cannot be nil")
	}
	return nil
}

func (s *Slack) NewAction(input ActionInput) *Action {
	if err := input.Validate(); err != nil {
		log.Println("bad action input:", err)
		return &Action{}
	}
	return &Action{
		Id:           input.Id,
		TeamId:       input.TeamId,
		TemplateName: input.TemplateName,
		TemplateMeta: s.getMeta(input.TemplateName, input.OnDemand),
		Data: TemplateData{
			EnvironmentParams: s.EnvParams(),
			Team:              input.Team,
			Channel:           input.Channel,
			User:              input.User,
			InputText:         input.InputText,
			InteractionData:   input.InteractionData,
			Timestamp:         time.Now().Unix(),
		},
	}
}

func (s *Slack) getMeta(templateName string, onDemand bool) TemplateMetadata {
	if onDemand {
		if meta, ok := s.onDemandMetadata[templateName]; ok {
			return *meta
		}
	} else {
		if meta, ok := s.templateMetadata[templateName]; ok {
			return *meta
		}
	}
	return TemplateMetadata{}
}

// ConvertCommandInput translates a string input into an Action
// this assumes the input is in a space delimited format that looks like a command line action with flags, stripping slack user ids
// example input: "action action2 -flag value -flag2 value2 -flag3"
// would be interpreted as attempting to process template "action_action2" with the map { "flag":"value", "flag2":"value2","flag3":true }
// *note* if template "action_action2" is not found, we will look for template "action"
func (s *Slack) ConvertCommandInput(teamId, teamDomain, channel, user, input string) (*Action, error) {
	args := strings.Fields(util.StripSlackUsers(input))
	tpl, inargs := util.FindTemplate(s.templates, args...)
	if tpl == nil {
		s.templateErrorsCounter.Add(1)
		return nil, fmt.Errorf("template not found in: %s", args)
	}
	return s.NewAction(ActionInput{
		Id:              "",
		TeamId:          teamId,
		Team:            teamDomain,
		Channel:         channel,
		User:            user,
		InputText:       input,
		InteractionData: util.ParseArgs(inargs),
		TemplateName:    tpl.Name(),
	}), nil
}

func (s *Slack) templateLookup(name string, onDemand bool) *template.Template {
	extra := name + ".tpl"
	var tpl *template.Template
	if onDemand {
		s.odtLock.Lock()
		defer s.odtLock.Unlock()
		tpl = s.onDemandTemplates.Lookup(name)
		if tpl == nil {
			tpl = s.onDemandTemplates.Lookup(extra)
		}
	} else {
		tpl = s.templates.Lookup(name)
		if tpl == nil {
			tpl = s.templates.Lookup(extra)
		}
	}
	if s.debug {
		log.Printf("template lookup:%s d:%t tpl:%T\n", name, onDemand, tpl)
	}
	return tpl
}

// ExecuteAction executes an Action which is typically processing a template or sending a message to kafka
// returns an ActionResult if further action is required, typically an ActionResult is returned if there is
// still something that needs to go back to slack.
func (s *Slack) ExecuteAction(action *Action) (*ActionResult, error) {
	tpl := s.templateLookup(action.TemplateName, action.OnDemand)
	if tpl == nil {
		s.templateErrorsCounter.Add(1)
		return nil, fmt.Errorf("invalid template: %s", action.TemplateName)
	}
	cloned, err := tpl.Clone() // Clone for request safety
	if err != nil {
		s.templateErrorsCounter.Add(1)
		return nil, fmt.Errorf("error cloning template [%s]: %v", tpl.Name(), err)
	}

	meta := s.getMeta(cloned.Name(), action.OnDemand)
	var rt ResponseType
	switch {
	case meta.IsTerminating:
		rt = None
	case meta.Dialog:
		rt = Dialog
	default:
		rt = action.ResponseType
	}

	buf := new(bytes.Buffer)
	if err := cloned.ExecuteTemplate(buf, cloned.Name(), action.Data); err != nil {
		s.templateErrorsCounter.Add(1)
		return nil, err
	}
	if s.debug {
		log.Println("TemplateResult:")
		log.Println(buf.String())
	}
	return &ActionResult{
		Action:            action,
		TeamId:            action.TeamId,
		SendToKafka:       meta.SendToKafka,
		KafkaMessageType:  KafkaMessageType(meta.KafkaMessageType),
		ResponseType:      rt,
		ResponseUrl:       action.ResponseUrl,
		Channel:           action.Channel,
		TriggerId:         action.TriggerId,
		ProcessedTemplate: buf.Bytes(),
		Data:              action.Data,
	}, nil
}

// TemplateData is intended to be fed into a template to give template authors
// system level settings along with the interaction data that comes back from interactions
type TemplateData struct {
	EnvironmentParams
	Team            string
	Channel         string
	User            string
	InputText       string
	Timestamp       int64
	InteractionData map[string]interface{}
}

// FeedbackMessage generates a FeedbackMessage object from the TemplateData object
func (t *TemplateData) FeedbackMessage() FeedbackMessage {
	var fbm FeedbackMessage
	value := t.InteractionData["value"]
	if str, ok := value.(string); ok {
		err := json.Unmarshal([]byte(str), &fbm)
		if err != nil {
			log.Println(err)
		}
	}
	fbm.User = t.User
	return fbm
}

// ParseMessage is convenience helper for translating a json []byte into a slack.Message
func ParseMessage(input []byte) (slack.Message, error) {
	var msg slack.Message
	if err := json.Unmarshal(input, &msg); err != nil {
		return msg, fmt.Errorf("failed unmarshaling %T: %s", msg, err)
	}
	return msg, nil
}

func ParseDialog(input []byte) (slack.Dialog, error) {
	var d slack.Dialog
	if err := json.Unmarshal(input, &d); err != nil {
		return d, fmt.Errorf("failed unmarshaling %T: %s", d, err)
	}
	return d, nil
}

func ParseInteractionCallback(b []byte) (slack.InteractionCallback, error) {
	var iacb slack.InteractionCallback
	if err := json.Unmarshal(b, &iacb); err != nil {
		return iacb, fmt.Errorf("failed unmarshaling %T: %s", iacb, err)
	}
	return iacb, nil
}
