package relay

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/atsu/goat/health"
	"github.com/gorilla/mux"
	"github.com/zserge/metric"
)

// Response is a container that encapsulates a response from a relayed request
type Response struct {
	Status int
	Body   []byte
}

var _ http.ResponseWriter = &Response{}
var _ io.Reader = &Response{}

func (r *Response) Header() http.Header {
	return http.Header{}
}

func (r *Response) Write(b []byte) (int, error) {
	r.Body = make([]byte, len(b))
	return copy(r.Body, b), nil
}

func (r *Response) Read(p []byte) (int, error) {
	// Is one shot body copy enough?
	// if not, we might need to track how many bytes were copied to pick up where
	// we left off in a subsequent call
	return copy(p, r.Body), io.EOF
}

func (r *Response) WriteHeader(statusCode int) {
	r.Status = statusCode
}

// Relayer is an internal object specifically for relaying requests via rpc.
// it must be exported to enable registering as an rpc
type Relayer struct {
	router   *mux.Router
	lastPing time.Time

	relayhook func()
}

// creating a new relayer should never happen outside of this package
// the relay hook allows the Relay to execute a function when the request is being relayed
// useful for counting the number of relayed requests
func newRelayer(hook func()) *Relayer {
	return &Relayer{
		router:    mux.NewRouter(),
		lastPing:  time.Unix(0, 0),
		relayhook: hook,
	}
}

func (r *Relayer) handleFunc(path string, handlerFunc http.HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, handlerFunc)
}

// Rpc call handle
const RelayerRelayRequest = "Relayer.RelayRequest"

// RelayRequest calls a local handler with a relayed request, this should only
// be called by the RelayHandler, the method signature should not change as it
// is required to satisfy the rpc.Register receiver input
func (r *Relayer) RelayRequest(req FauxRequest, response *Response) error {
	r.relayhook()
	match := &mux.RouteMatch{}
	if r.router.Match(req.Request(), match) {
		match.Handler.ServeHTTP(response, req.Request())
	} else {
		return match.MatchErr
	}
	return nil
}

// Rpc call handle
const RelayerPing = "Relayer.Ping"

// Ping is used to check if the Relay is still isConnected
func (r *Relayer) Ping(t time.Time, recv *string) error {
	r.lastPing = t
	*recv = "pong"
	return nil
}

// RelayMode describes the supported modes a Relay can be in
type RelayMode string

func (r RelayMode) String() string {
	switch {
	case r == OFF:
		return "off"
	default:
		return string(r)
	}
}

const (
	// OFF if the relay is created but not used
	OFF = RelayMode("")

	// PassThrough describes the mode in which, requests are accepted and forwarded on to a handler relay
	PassThrough = RelayMode("passthrough")

	// Handler mode is when the relay is accepting requests and processing the requests as if they came in from an HTTP server
	Handler = RelayMode("handle")
)

// Relay
type Relay struct {
	Host      string
	Port      int
	WhiteList *regexp.Regexp
	Mode      RelayMode
	Debug     bool
	conf      *tls.Config

	relayer *Relayer

	checkInterval time.Duration

	rpcsCounter    metric.Metric
	connectCounter metric.Metric
	pingHistogram  metric.Metric
	rpcHistogram   metric.Metric

	lock            sync.Mutex
	isConnected     bool
	connectTimes    []int64
	disconnectTimes []int64
	connectTime     int64
	connectCnt      int64

	doneCh    chan struct{}
	rpcClient *rpc.Client
	rpcServer *rpc.Server
}

func NewRelay(host string, port int, whitelist string, mode RelayMode, conf *tls.Config) *Relay {

	r := &Relay{
		Host:      host,
		Port:      port,
		WhiteList: regexp.MustCompile(whitelist),
		Mode:      mode,
		conf:      conf,

		connectTimes:    make([]int64, 0, 100),
		disconnectTimes: make([]int64, 0, 100),
		rpcsCounter:     metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		connectCounter:  metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
		pingHistogram:   metric.NewHistogram("1h1h"), // 1 hour history, 1 hour precision
		rpcHistogram:    metric.NewHistogram("1h1h"), // 1 hour history, 1 hour precision

		checkInterval: time.Second * 2,
		doneCh:        make(chan struct{}),
	}
	return r
}

// Init is used to initialize the relay
func (r *Relay) Init() error {
	r.relayer = newRelayer(func() { r.rpcsCounter.Add(1) })
	if r.Mode == Handler {
		r.rpcServer = rpc.NewServer()
		if err := r.rpcServer.Register(r.relayer); err != nil {
			// we should never get here, if we do this code is self destructing
			// and there is no way to recover
			return err
		}
	}
	return nil
}

func (r *Relay) SetDebug(b bool) {
	r.Debug = b
}

func (r *Relay) SetCheckInterval(duration time.Duration) {
	r.checkInterval = duration
}

func (r *Relay) LastPingTime() time.Time {
	if r.relayer != nil {
		return r.relayer.lastPing
	}
	return time.Time{}
}

// RelayStatus describes the health of the relay
type RelayStatus struct {
	Health          health.State `json:"health"`
	Mode            string       `json:"mode"`
	LastPingTime    int64        `json:"lastPingTime,omitempty"`
	Connected       bool         `json:"isConnected"`
	ConnectTime     int64        `json:"connectTime,omitempty"`
	ConnectCount    int64        `json:"connectCount,omitempty"`
	RpcCount        int64        `json:"rpcCount,omitempty"`
	RpcsCounter     interface{}  `json:"rpcsCounter,omitempty"`
	ConnectCounter  interface{}  `json:"connectCounter,omitempty"`
	RpcLatencySecs  interface{}  `json:"rpcLatencySecs,omitempty"`
	PingLatencySecs interface{}  `json:"pingLatencySecs,omitempty"`
}

// Status create and return a RelayStatus for the current status
func (r *Relay) Status() RelayStatus {
	h := health.Green
	if !r.isConnected && r.Mode != OFF {
		h = health.Red
	}
	status := RelayStatus{
		Health:         h,
		Mode:           r.Mode.String(),
		LastPingTime:   r.LastPingTime().Unix(),
		Connected:      r.isConnected,
		ConnectTime:    r.connectTime,
		RpcsCounter:    r.rpcsCounter,
		ConnectCounter: r.connectCounter,
		ConnectCount:   r.connectCnt,
	}
	if r.Mode == PassThrough {
		status.RpcLatencySecs = r.rpcHistogram
		status.PingLatencySecs = r.pingHistogram
	}
	return status
}

// HandleFunc should be used in place of *Router.HandleFunc, to register
// handlers to accept relayed events.
func (r *Relay) HandleFunc(path string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	return r.relayer.handleFunc(path, f)
}

// RelayHandler takes the place of an actual handler to glue an existing http server to the Relay
// invoking this handler will forward the request directly to a relay in handle mode
func (r *Relay) RelayHandler(w http.ResponseWriter, req *http.Request) {
	if r.Debug {
		log.Println("Relaying Request from:", req.URL.String())
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	faux := CreateFauxRequest(req, body)
	resp := &Response{}
	if r.rpcClient == nil {
		err = errors.New("relay failed, no rpc client")
	} else {
		start := time.Now()
		err = r.rpcClient.Call(RelayerRelayRequest, faux, &resp)
		if err != nil {
			log.Println(err)
		}
		r.rpcsCounter.Add(1)

		duration := time.Since(start)
		r.rpcHistogram.Add(duration.Seconds())
		if r.Debug {
			log.Println("RPC took:", duration.String())
		}
	}
	if err != nil {
		log.Println(err)
		http.Error(w, "relay failure", http.StatusInternalServerError)
		return
	}
	if resp.Status == 0 {
		resp.Status = 200
	}
	w.WriteHeader(resp.Status)
	if _, err := w.Write(resp.Body); err != nil {
		log.Println(err)
	}
}

// Listen opens the configured port and accepts a single tcp connection, if the connection drops
// this will start listening again for another connection.
func (r *Relay) Listen() error {
	lsc, err := tls.Listen("tcp", fmt.Sprintf(":%d", r.Port), r.conf)
	if err != nil {
		return err
	}

	go func() {
		<-r.doneCh
		if err := lsc.Close(); err != nil {
			log.Println(err)
		}
	}()

	go func() {
		for {
			conn, err := lsc.Accept()
			if err != nil {
				return
			} else {
				r.handleConn(conn)
			}
		}
	}()
	return nil
}

// Connect makes an outgoing request to connected to a listening Relay
func (r *Relay) Connect() {
	go func() {
		retry := 0
		sleepDur := time.Duration(0)
		for {
			select {
			case <-r.doneCh:
				return
			default:
			}
			// basic back off logic max of 5s
			if retry > 0 {
				log.Printf("sleeping %s to retry\n", sleepDur)
				time.Sleep(sleepDur)
			}
			target := fmt.Sprintf("%s:%d", r.Host, r.Port)
			conn, err := tls.Dial("tcp", target, r.conf)
			if err != nil {
				if retry < 5 {
					sleepDur += 500 * time.Millisecond * time.Duration(retry)
					retry++
				}
				log.Printf("failed to connect to %q - %v", target, err)
				continue
			}
			retry = 0
			sleepDur = time.Duration(0)
			r.handleConn(conn)
		}
	}()
}

// AddrAllowed takes an address and returns whether that address is allowed by Relay's whitelist
// Only invoked when the relay is in Passthrough mode, and discards the port
func (r *Relay) AddrAllowed(addr net.Addr) bool {
	if tcpAdr, ok := addr.(*net.TCPAddr); ok {
		return r.WhiteList.MatchString(tcpAdr.IP.String())
	}
	return false
}

// handleConn blocks while either rpc client or server are isConnected.
func (r *Relay) handleConn(conn net.Conn) {
	r.connected()
	log.Println("connected to:", conn.RemoteAddr())
	if r.Mode == PassThrough {
		if r.AddrAllowed(conn.RemoteAddr()) {
			r.rpcClient = rpc.NewClient(conn)
			<-r.clientConnectionWatcher() // block until rpc client appears to be disconnected
		} else {
			log.Printf("ip %q failed to match whitelist %q\n", conn.RemoteAddr().String(), r.WhiteList.String())
		}
	} else {
		// ServeConn in go routine because it blocks, Closes when the client hangs up
		r.rpcServer.ServeConn(conn)
	}
	r.disconnected()
}

func (r *Relay) disconnected() {
	r.lock.Lock()
	defer r.lock.Unlock()
	if len(r.connectTimes) > 1000 {
		r.disconnectTimes = r.disconnectTimes[1:]
	}
	r.disconnectTimes = append(r.disconnectTimes, time.Now().Unix())
	log.Print("disconnected")
	r.isConnected = false
}

func (r *Relay) connected() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.connectCnt++
	r.connectTime = time.Now().Unix()
	r.isConnected = true
	r.connectCounter.Add(1)

	if len(r.connectTimes) > 1000 {
		r.connectTimes = r.connectTimes[1:]
	}
	r.connectTimes = append(r.connectTimes, time.Now().Unix())
	r.isConnected = true
}

// clientConnectionWatcher returns a channel that will be closed, if the connection
// appears to be disconnected. To check, we continuously call Ping via rpc to verify
// our connection is still alive.
func (r *Relay) clientConnectionWatcher() <-chan struct{} {
	out := make(chan struct{})
	go func() {
		for {
			select {
			case <-r.doneCh:
				close(out)
				return
			case <-time.After(r.checkInterval):
				switch {
				case r.Mode == Handler:
					if time.Since(r.LastPingTime()) > (r.checkInterval * 2) {
						if r.Debug {
							log.Println("Ping not seen...")
						}
						close(out)
						return
					}
				case r.Mode == PassThrough:
					if r.rpcClient != nil {
						start := time.Now()
						r.relayer.lastPing = start
						var resp *string
						err := r.rpcClient.Call(RelayerPing, start, &resp)
						if err != nil || resp == nil {
							log.Println("ping failed resp:", resp, "error:", err)
							close(out)
							return
						}
						r.pingHistogram.Add(time.Since(start).Seconds())
					}
				default:
					// unknown mode, bail out
					log.Println("Unknown mode:", r.Mode)
					close(out)
					return
				}
			}
		}
	}()
	return out
}

// Close
func (r *Relay) Close() {
	if r.rpcClient != nil {
		if err := r.rpcClient.Close(); err != nil {
			log.Println(err)
		}
	}
	close(r.doneCh)
}

// FauxRequest is the same as an http.Request, but since we can't send an io.Reader
// The body is a pre-read byte array
type FauxRequest struct {
	Method           string
	URL              *url.URL
	Proto            string // "HTTP/1.0"
	ProtoMajor       int    // 1
	ProtoMinor       int    // 0
	Header           http.Header
	Body             []byte // io.ReadCloser
	ContentLength    int64
	TransferEncoding []string
	Host             string
	Form             url.Values
}

func CreateFauxRequest(req *http.Request, body []byte) *FauxRequest {
	return &FauxRequest{
		Method:           req.Method,
		URL:              req.URL,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           req.Header,
		Body:             body,
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Host:             req.Host,
		Form:             req.Form,
	}
}

// Request turns the FauxRequest into an *http.Request
func (f *FauxRequest) Request() *http.Request {
	return &http.Request{
		Method:           f.Method,
		URL:              f.URL,
		Proto:            f.Proto,
		ProtoMajor:       f.ProtoMajor,
		ProtoMinor:       f.ProtoMinor,
		Header:           f.Header,
		Body:             ioutil.NopCloser(bytes.NewReader(f.Body)),
		ContentLength:    f.ContentLength,
		TransferEncoding: f.TransferEncoding,
		Host:             f.Host,
		Form:             f.Form,
	}
}
