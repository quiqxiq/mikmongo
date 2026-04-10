package producer

import (
	"context"
	"encoding/json"

	"mikmongo/pkg/rabbitmq"
)

// SuspendProducer handles suspend-related messages
type SuspendProducer struct {
	client *rabbitmq.Client
}

// SuspendCustomerEvent represents a suspend customer event
type SuspendCustomerEvent struct {
	CustomerID string `json:"customer_id"`
	RouterID   string `json:"router_id"`
	Reason     string `json:"reason"`
}

// NewSuspendProducer creates a new suspend producer
func NewSuspendProducer(client *rabbitmq.Client) *SuspendProducer {
	return &SuspendProducer{client: client}
}

// PublishSuspendCustomer publishes a suspend customer event
func (p *SuspendProducer) PublishSuspendCustomer(ctx context.Context, event *SuspendCustomerEvent) error {
	if p.client == nil {
		return errRabbitMQNotConfigured
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, "suspend.exchange", "customer.suspend", body)
}
