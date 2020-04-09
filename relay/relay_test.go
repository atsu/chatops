package relay

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/atsu/goat/util"
)

func generateTlsConfig(t *testing.T) *tls.Config {
	t.Helper()
	cert, err := tls.LoadX509KeyPair("testdata/test.crt", "testdata/test.key")
	if err != nil {
		t.Error(err)
	}
	return &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}
}

func TestRelayTest(t *testing.T) {
	conf := generateTlsConfig(t)
	src := NewRelay("", 5000, ".+", PassThrough, conf)
	dst := NewRelay("", 5000, ".+", Handler, conf)
	src.Debug = true
	dst.Debug = true
	if err := src.Init(); err != nil {
		t.Error(err)
	}
	if err := dst.Init(); err != nil {
		t.Error(err)
	}

	// Server listens for connection
	go func() {
		if err := src.Listen(); err != nil {
			log.Println(err)
		}
	}()
	time.Sleep(time.Millisecond * 500) // let the server start

	headerKey, headerVal := util.RandomString(10), util.RandomString(10)
	reqBody := []byte(util.RandomString(10))
	respBody := []byte(util.RandomString(10))
	respStatus := 200
	// Replayer created with a handler
	dst.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, headerVal, request.Header.Get(headerKey))
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, reqBody, body)
		writer.WriteHeader(respStatus)
		if n, err := writer.Write(respBody); err != nil {
			t.Error(err)
		} else {
			assert.Equal(t, len(respBody), n)
		}
	})
	// Connect to server with the Relayer
	go dst.Connect()
	for i := 0; i < 100; i++ {
		if src.isConnected && dst.isConnected {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	// RelayHandler on the server side is hit
	reader := bytes.NewReader(reqBody)
	req, err := http.NewRequest("GET", "/", reader)
	if err != nil {
		t.Error(err)
	}
	req.Header.Add(headerKey, headerVal)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(src.RelayHandler)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, respStatus, rr.Code)
	assert.Equal(t, respBody, rr.Body.Bytes())

	time.AfterFunc(5*time.Millisecond, func() {
		dst.Close()
		src.Close()
	})

	<-dst.doneCh
}

func TestCancel(t *testing.T) {
	conf := generateTlsConfig(t)
	src := NewRelay("", 5000, ".+", PassThrough, conf)
	dst := NewRelay("", 5000, ".+", Handler, conf)
	if err := src.Init(); err != nil {
		t.Error(err)
	}
	if err := dst.Init(); err != nil {
		t.Error(err)
	}
	time.AfterFunc(time.Millisecond*500, func() {
		dst.Close()
		src.Close()
		assert.False(t, src.isConnected)
		assert.False(t, dst.isConnected)
	})

	assert.False(t, src.isConnected)
	assert.False(t, dst.isConnected)
	go func() {
		if err := src.Listen(); err != nil {
			t.Error(err)
		}
	}()
	go dst.Connect()
	for i := 0; i < 100; i++ {
		if src.isConnected && dst.isConnected {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	assert.True(t, src.isConnected)
	assert.True(t, dst.isConnected)
}

func TestRelay_IsWhiteListed(t *testing.T) {
	tests := []struct {
		name          string
		ip            string
		whitelist     string
		isWhiteListed bool
	}{
		{"default - all allowed", "1.2.3.4", ".+", true},
		{"explicit match allowed", "1.2.3.4", "1.2.3.4", true},
		{"explicit match disallowed", "2.2.3.4", "1.2.3.4", false},
		{"range allowed", "1.2.3.100", `1.2.3.\d+`, true},
		{"range disallowed", "1.2.4.100", `1.2.3.\d+`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			relay := NewRelay("", 0, test.whitelist, PassThrough, generateTlsConfig(t))
			addr := &net.TCPAddr{IP: net.ParseIP(test.ip)}
			assert.Equal(t, test.isWhiteListed, relay.AddrAllowed(addr))
		})
	}
}
