package producer

import (
	"context"
	"encoding/json"

	"mikmongo/pkg/rabbitmq"
)

// NotificationProducer handles notification messages
type NotificationProducer struct {
	client *rabbitmq.Client
}

// NotificationEvent represents a notification event
type NotificationEvent struct {
	Type    string `json:"type"` // email, wa, sms
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// NewNotificationProducer creates a new notification producer
func NewNotificationProducer(client *rabbitmq.Client) *NotificationProducer {
	return &NotificationProducer{client: client}
}

// PublishNotification publishes a notification event
func (p *NotificationProducer) PublishNotification(ctx context.Context, event *NotificationEvent) error {
	if p.client == nil {
		return errRabbitMQNotConfigured
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, "notification.exchange", "notification.send", body)
}
