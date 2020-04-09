package app

import (
	"sync"
	"testing"
	"time"

	"github.com/atsu/chatops/bot"
	"github.com/atsu/chatops/relay"
	"github.com/atsu/goat/health"
	"github.com/atsu/goat/health/mocks"
	smock "github.com/atsu/goat/stream/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewChatOps(t *testing.T) {
	co := NewChatOps("test")
	assert.Equal(t, "test", co.Info.Component)
	assert.NotNil(t, co.sc)
	assert.NotNil(t, co.router)
	assert.NotNil(t, co.doneCh)
	assert.NotNil(t, co.kafkaCh)
}

func Test(t *testing.T) {
	resCh := make(chan int, 1)
	sMock := new(smock.KafkaStreamConfig)
	sMock.On("FullTopic", mock.AnythingOfType("string")).Return("topic")
	sMock.On("Produce", mock.AnythingOfType("*string"), mock.MatchedBy(func(b []byte) bool {
		resCh <- 1
		return "message" == string(b)
	})).Return(nil)
	sMock.On("NewProducer", mock.Anything).Return(nil, nil)
	co := NewChatOps("test")
	defer func() { close(co.doneCh) }()
	co.sc = sMock
	co.Kafka = true
	co.InitKafka()
	co.KafkaProduce("abc", "message")

	select {
	case <-resCh:
	case <-time.After(time.Millisecond * 100):
		t.Fail()
	}
	sMock.AssertExpectations(t)
}

func TestChatOps_StartStatusUpdater(t *testing.T) {
	tests := []struct {
		name  string
		stats []string
		mode  relay.RelayMode
	}{
		{"off status metrics", []string{"slack", "helpers"}, relay.OFF},
		{"handler status metrics", []string{"relay", "slack", "helpers"}, relay.Handler},
		{"passthrough handler metrics", []string{"relay"}, relay.PassThrough},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock := sync.Mutex{}
			stats := make([]string, 0)
			hrmock := new(mocks.IReporter)
			hrmock.On("Health").Return(health.Event{})
			hrmock.On("SetHealth", mock.AnythingOfType("health.State"), mock.AnythingOfType("string"))
			hrmock.On("AddStat", mock.AnythingOfType("string"), mock.Anything).Return().
				Run(func(args mock.Arguments) {
					lock.Lock()
					arg := args.Get(0).(string)
					stats = append(stats, arg)
					lock.Unlock()
				})
			r := &relay.Relay{Mode: test.mode}
			cho := ChatOps{
				relay:  r,
				sl:     &bot.Slack{},
				hr:     hrmock,
				doneCh: make(chan int)}
			cho.StartStatusUpdater(time.Millisecond * 100)
			<-time.After(time.Millisecond * 150)
			close(cho.doneCh)
			for _, s := range test.stats {
				assert.Contains(t, stats, s)
			}
		})
	}
}
