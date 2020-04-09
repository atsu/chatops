package interfaces

import "github.com/atsu/goat/health"

// ChatOpsCom provides a communication interface for sub components
type ChatOpsCom interface {
	KafkaProduce(topic, message string)
	EnvironmentParams() map[string]string
}

type ChatOpsClient interface {
	Health() (health.Event, error)
	SlackAtsuEvent(templateName string, fields map[string]string) error
}
