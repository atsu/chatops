package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atsu/chatops/app"
	"github.com/atsu/chatops/bot"
	"github.com/atsu/chatops/interfaces"
	"github.com/atsu/goat/health"
	"github.com/stretchr/testify/assert"
)

var _ interfaces.ChatOpsClient = &Client{}

func TestClient_EndpointVerify(t *testing.T) {
	// Verify the actual endpoints match the client endpoint.
	// Need to do this because of the package structure, we can't just export bot.AtsuEventEndpoint
	// and pull it into the client package because then a downstream user won't be able to find
	// the `chatops/bot` package since they would need the full `github.com/chatops/bot` reference.

	assert.Equal(t, bot.AtsuEventEndpoint, atsuEventEndpoint)
	assert.Equal(t, app.HealthEndpoint, healthEndpoint)
}

func TestClient_SendAtsuEvent(t *testing.T) {
	tpl := "_health_change"
	parameters := map[string]string{
		"health": "blue",
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		rw.Write([]byte(`{"status":"ok"}`))

		assert.Equal(t, req.URL.String(), fmt.Sprintf("%s?tpl=%s", atsuEventEndpoint, tpl))
		assert.Equal(t, parameters, jsonToMap(t, body))
	}))
	defer server.Close()

	cl := NewClient(server.URL)
	err := cl.SlackAtsuEvent(tpl, parameters)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_Health(t *testing.T) {
	hevent := health.Event{
		Hostname:  "host",
		Timestamp: 0,
		Type:      "status",
		Name:      "health",
		Service:   "testing",
		Version:   "1",
		State:     "blue",
		Message:   "test",
		Data:      "data",
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		out, err := json.Marshal(hevent)
		if err != nil {
			t.Fatal(err)
		}
		rw.Write(out)

		assert.Equal(t, req.URL.String(), app.HealthEndpoint)
	}))

	cl := NewClient(server.URL)
	h, err := cl.Health()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Health:", h)
	assert.Equal(t, hevent, h)
}

func jsonToMap(t *testing.T, data []byte) map[string]string {
	t.Helper()
	params := make(map[string]string)
	if err := json.Unmarshal(data, &params); err != nil {
		t.Fatal(err)
	}
	return params
}
