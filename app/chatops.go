package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atsu/chatops/db"

	"github.com/atsu/chatops/bot"
	"github.com/atsu/chatops/interfaces"
	"github.com/atsu/chatops/relay"
	"github.com/atsu/chatops/util"
	"github.com/atsu/goat/build"
	"github.com/atsu/goat/health"
	"github.com/atsu/goat/stream"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
)

const (
	HealthEndpoint         = "/health"
	RequestTrackingHistory = 1000
)

var _ interfaces.ChatOpsCom = &ChatOps{}

type nopCom struct {
}

func (n *nopCom) KafkaProduce(topic, message string) {
	log.Printf("no op com topic: [%s] msg: %s\n", topic, message)
}

func (n *nopCom) EnvironmentParams() map[string]string {
	return map[string]string{}
}

// TODO:(smt) Config to toggle OnDemandTemplates
type ChatOps struct {
	Kafka                  bool   `envconfig:"KAFKA"`
	SlackAppId             string `envconfig:"SLACK_APP_ID"`
	SlackVerificationToken string `envconfig:"SLACK_VFY_TOKEN"`
	SlackSecretSigningKey  string `envconfig:"SLACK_SECRET_SIGNING_KEY"`
	SlackClientId          string `envconfig:"SLACK_CLIENT_ID"`
	SlackClientSecret      string `envconfig:"SLACK_CLIENT_SECRET"`
	SlackAuthRedirectUrl   string `envconfig:"SLACK_AUTH_REDIRECT_URL"`
	//SlackToken             string `envconfig:"SLACK_TOKEN"`
	//SlackInHook            string `envconfig:"SLACK_IN_HOOK"`
	FeedbackTopic    string `envconfig:"FEEDBACK_TOPIC"`
	Port             int    `envconfig:"PORT"`
	ElasticSearchUrl string `envconfig:"ELASTICSEARCH_URL"`
	ViewUrl          string `envconfig:"VIEW_URL"`
	HealthUrl        string `envconfig:"HEALTH_URL"`
	TemplateDir      string `envconfig:"TEMPLATE_DIR"`
	RelayPort        int    `envconfig:"RELAY_PORT"`
	RelayHost        string `envconfig:"RELAY_HOST"`
	RelayPassthrough bool   `envconfig:"RELAY_PASSTHROUGH"`
	RelayHandler     bool   `envconfig:"RELAY_HANDLER"`
	RelayCertFile    string `envconfig:"RELAY_CERT_FILE"`
	RelayKeyFile     string `envconfig:"RELAY_KEY_FILE"`
	RelayInsecure    bool   `envconfig:"RELAY_INSECURE"`
	RelayWhiteList   string `envconfig:"RELAY_WHITELIST"`
	DbFile           string `envconfig:"DB_FILE"`
	Debug            bool   `envconfig:"DEBUG"`

	sc stream.KafkaStreamConfig

	Info     build.Info
	router   *mux.Router
	relay    *relay.Relay
	sl       *bot.Slack
	hr       health.IReporter
	database db.Database
	doneCh   chan int
	kafkaCh  chan KafkaMessage
}

func NewChatOps(name string) *ChatOps {
	return &ChatOps{
		Info:    build.GetInfo(name),
		sc:      &stream.StreamConfig{},
		router:  mux.NewRouter(),
		doneCh:  make(chan int),
		kafkaCh: make(chan KafkaMessage),
	}
}

func (c *ChatOps) SetFlags() {
	flag.BoolVar(&c.Kafka, "kafka", true, "enable or disable kafka")
	flag.StringVar(&c.SlackAppId, "sapp", "", "slack app id")
	flag.StringVar(&c.SlackVerificationToken, "svt", "", "slack verification token")
	//flag.StringVar(&c.SlackToken, "st", "", "slack bot token")
	//flag.StringVar(&c.SlackInHook, "sin", "", "slack incoming web hook url")
	flag.StringVar(&c.SlackSecretSigningKey, "ssk", "", "slack secret signing key")
	flag.StringVar(&c.SlackClientId, "sci", "", "slack client id")
	flag.StringVar(&c.SlackClientSecret, "scs", "", "slack client secret")
	flag.StringVar(&c.SlackAuthRedirectUrl, "sru", "https://slack.com/", "slack redirect url after successful oauth")
	flag.StringVar(&c.FeedbackTopic, "ft", "slack", "kafka topic on which to send feedback interactions")
	flag.StringVar(&c.ElasticSearchUrl, "es", "", "elastic search url")
	flag.StringVar(&c.ViewUrl, "view", "", "view url")
	flag.StringVar(&c.HealthUrl, "health", "https://health.atsu.io", "health url")
	flag.StringVar(&c.TemplateDir, "d", "./templates", "root template directory")
	flag.StringVar(&c.RelayHost, "rhost", "", "target passthrough relay host to connect to when in handler mode")
	flag.IntVar(&c.RelayPort, "rport", 5000, "relay communications port")
	flag.BoolVar(&c.RelayPassthrough, "rp", false, "relay pass through mode")
	flag.BoolVar(&c.RelayHandler, "rh", false, "relay handler mode")
	flag.StringVar(&c.RelayCertFile, "cert", "cert.crt", "x509 key pair cert file")
	flag.StringVar(&c.RelayKeyFile, "key", "cert.key", "x509 key pair key file")
	flag.BoolVar(&c.RelayInsecure, "insecure", false, "tls skip verify")
	flag.StringVar(&c.RelayWhiteList, "whitelist", ".+", "apply whitelist filter to addresses connecting to the relay (passthrough only)")
	flag.BoolVar(&c.Debug, "debug", false, "verbose output")
	flag.StringVar(&c.DbFile, "db", "./chatops.db", "database target file")

	flag.IntVar(&c.Port, "port", 8040, "port for status api.")

	if err := envconfig.Process("ATSU", c); err != nil {
		log.Fatal(err) // Don't allow startup if there is env value confusion.
	}
	c.sc.SetFlags()
}

func (c *ChatOps) StartSignalHandler() {
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-ch
		log.Println("shutting down")
		close(c.doneCh)
	}()
}

type KafkaMessage struct {
	Topic   string
	Message string
}

func (k KafkaMessage) String() string {
	b, _ := json.Marshal(k)
	return string(b)
}

// Run is the main entry point for ChatOps
func (c *ChatOps) Run() {
	c.SetFlags()
	version := flag.Bool("version", false, "chatops version")
	flag.Parse()
	fmt.Println(c.Info.Banner())
	if *version {
		return
	}

	if c.Debug {
		b, _ := json.Marshal(c)
		log.Println(string(b))
	}
	if c.Kafka {
		log.Println("brokers:", c.sc.GetBrokers())
		log.Println("prefix:", c.sc.GetPrefix())
	} else {
		log.Println("kafka disabled")
	}
	c.InitKafka()
	c.InitDb()
	c.InitRelay()
	c.InitHealth(c.sc.GetPrefix(), c.sc.GetBrokers())
	c.InitSlack()

	c.StartSignalHandler()
	c.StartRelay()
	c.StartServer()

	c.StartStatusUpdater(time.Second * 30)
	<-c.doneCh

	if err := c.hr.Stop(); err != nil {
		log.Println(err)
	}
	c.relay.Close()
	c.sl.Stop()
}

func (c *ChatOps) InitDb() {
	// TODO:(smt) Encrypt db file doesn't work?
	dbUser := "dbadmin123"
	dbPass := "sdlkjerwoeifjsldffneflsefwiejf3jwlkj3"
	log.Println("loading database:", c.DbFile)
	c.database = db.NewSqliteDB(fmt.Sprintf("file:%s?_auth&_auth_user=%s&_auth_pass=%s", c.DbFile, dbUser, dbPass))
	if err := c.database.Init(); err != nil {
		log.Fatal(err)
	}
}

func (c *ChatOps) InitKafka() {
	if !c.Kafka {
		return
	}
	_, err := c.sc.NewProducer(nil)
	if err != nil {
		log.Println(err)
	}
	c.monitorKafkaChan()
}

// monitorKafkaChan starts a go thread that listens to kafkaCh and produces messages to kafka
func (c *ChatOps) monitorKafkaChan() {
	go func() {
		for {
			select {
			case <-c.doneCh:
				return
			case m := <-c.kafkaCh:
				topic := c.sc.FullTopic(fmt.Sprintf("chatops.%s", m.Topic))
				if err := c.sc.Produce(&topic, []byte(m.Message)); err != nil {
					log.Printf("failed to send to kafka topic: %q msg: %q\n", topic, m)
				}
			}
		}
	}()
}

// KafkaProduce is a mechanism for allowing subcomponents to send messages to kafka, without exposing the underlying channel
func (c *ChatOps) KafkaProduce(topic, message string) {
	go func() { c.kafkaCh <- KafkaMessage{topic, message} }()
}

func (c *ChatOps) EnvironmentParams() map[string]string {
	m := make(map[string]string)
	m["ElasticSearchUrl"] = c.ElasticSearchUrl
	m["ViewUrl"] = c.ViewUrl
	m["HealthUrl"] = c.HealthUrl
	return m
}

// InitHealth creates, initializes, and starts the health reporter
func (c *ChatOps) InitHealth(prefix, brokers string) {
	c.hr = health.NewReporter(c.Info.Component, prefix, brokers, func(err error) {
		fmt.Println("Health Reporter Error:", err)
	})
	if c.relay.Mode != relay.OFF {
		c.hr.AddHostnameSuffix(string(c.relay.Mode))
	}
	c.hr.SetStdOutFallback(true)
	c.router.HandleFunc("/", c.hr.HealthHandler)
	c.router.HandleFunc(HealthEndpoint, c.hr.HealthHandler)
	if !c.Kafka {
		return
	}
	c.hr.SetHealth(health.Blue, "starting")
	ok, err := c.hr.Initialize()
	if !ok {
		log.Fatal(err)
	}
	c.hr.StartIntervalReporting(time.Minute)
}

// InitSlack initializes the slack sub-component
func (c *ChatOps) InitSlack() {
	var com interfaces.ChatOpsCom
	if c.Kafka {
		com = c
	} else {
		com = &nopCom{}
	}
	cfg := bot.SlackConfig{
		//Token:             c.SlackToken,
		//InWebHook:         c.SlackInHook,
		AppId:             c.SlackAppId,
		SecretSigningKey:  c.SlackSecretSigningKey,
		VerificationToken: c.SlackVerificationToken,
		ClientId:          c.SlackClientId,
		ClientSecret:      c.SlackClientSecret,
		AuthRedirectUrl:   c.SlackAuthRedirectUrl,
		FeedbackTopic:     c.FeedbackTopic,
		TemplateDir:       c.TemplateDir,
	}
	// TODO: cfg.Validate()
	c.sl = bot.NewSlack(cfg, com, c.database)
	c.sl.SetDebug(c.Debug)
	if err := c.sl.Start(c.router, c.relay); err != nil {
		log.Fatal("failed to initialize slack:", err)
	}
}

func (c *ChatOps) StartRelay() {
	log.Printf("starting relay, mode: %s\n", c.relay.Mode)
	if c.RelayInsecure {
		log.Println("relay insecure mode is ON")
	}
	target := fmt.Sprintf("%s:%d", c.RelayHost, c.RelayPort)
	if c.RelayPassthrough {
		log.Printf("listening for relay connection on %q\n", target)
		log.Printf("passthrough whitelist: %q\n", c.RelayWhiteList)
		if err := c.relay.Listen(); err != nil {
			log.Fatal(err)
		}
	}
	if c.RelayHandler {
		log.Printf("connecting to relay %q\n", target)
		c.relay.Connect()
	}
}

func (c *ChatOps) InitRelay() {
	if c.RelayHandler && c.RelayPassthrough {
		log.Fatal("relay handler and passthrough modes are mutually exclusive.")
	}
	conf := &tls.Config{InsecureSkipVerify: c.RelayInsecure}
	var mode relay.RelayMode
	switch {
	case c.RelayHandler:
		mode = relay.Handler
		// conf.ServerName = c.RelayHost // Do we want SNI? (https://en.wikipedia.org/wiki/Server_Name_Indication)
	case c.RelayPassthrough:
		mode = relay.PassThrough
		cf := util.GetAbsoluteFilePath(c.RelayCertFile)
		kf := util.GetAbsoluteFilePath(c.RelayKeyFile)
		certs, err := tls.LoadX509KeyPair(cf, kf)
		if err != nil {
			log.Println("error loading X509KeyPair:", err)
		}
		conf.Certificates = []tls.Certificate{certs}
	}
	c.relay = relay.NewRelay(c.RelayHost, c.RelayPort, c.RelayWhiteList, mode, conf)
	if err := c.relay.Init(); err != nil {
		log.Fatalf("relay init failed mode: %s err: %v", mode, err)
	}
	c.relay.SetDebug(c.Debug)
}

// StartStatusUpdater provides a continual loop for updating application health data.
func (c *ChatOps) StartStatusUpdater(interval time.Duration) {
	go func() {
		for {
			select {
			case <-c.doneCh:
				return
			case <-time.After(interval):
				h := health.Green
				msg := ""
				rstatus := c.relay.Status()
				if relay.RelayMode(rstatus.Mode) != relay.OFF {
					c.hr.AddStat("relay", rstatus)
					if rstatus.Health != h {
						h = rstatus.Health
					}
				}
				if c.relay.Mode != relay.PassThrough && c.sl != nil {
					c.hr.AddStat("slack", c.sl.Status())
					c.hr.AddStat("helpers", bot.HelperMetrics.Status())
				}
				c.hr.SetHealth(h, msg)
				if c.Debug {
					out, _ := json.Marshal(c.hr.Health())
					log.Println(string(out))
				}
			}
		}
	}()
}

// ChatOpsHandler handles requests to the general service.
func (c *ChatOps) ChatOpsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	switch vars["action"] {
	case "info":
		if r.Method == http.MethodGet {
			if _, err := w.Write([]byte(c.Info.String())); err != nil {
				log.Println(err)
			}
		}
	case "reload":
		if r.Method == http.MethodPost {

			response := "reloaded"

			err := c.sl.LoadTemplates()
			if err != nil {
				response = err.Error()
			}

			if _, err := w.Write([]byte(response)); err != nil {
				log.Println(err)
			}
		}
	}
}

func (c *ChatOps) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rb, err := httputil.DumpRequest(r, false)
		if err != nil {
			fmt.Println("logging request failed", err)
			return
		}
		fields := strings.Fields(string(rb))
		log.Println(fields)

		next.ServeHTTP(w, r)
	})
}

// StartServer starts the application web server
func (c *ChatOps) StartServer() {
	c.router.Use(c.loggingMiddleware)
	c.router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	c.router.HandleFunc("/chatops/{action}", c.ChatOpsHandler)
	server := http.Server{
		Handler: c.router,
		Addr:    fmt.Sprintf(":%d", c.Port),
	}

	if err := c.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		if route.GetHandler() != nil { // only list routes with a handler
			tmpl, _ := route.GetPathTemplate()
			log.Println("active route:", tmpl)
		}
		return nil
	}); err != nil {
		log.Println(err)
	}

	// listen for done and shutdown the server
	go func() {
		<-c.doneCh
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.Shutdown(ctx); err != nil {
			log.Println(err)
		}
		log.Println("http server shutting down")
	}()

	// start the server, without blocking
	go func() {
		log.Printf("starting http server: %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println(err)
		}
	}()
}
