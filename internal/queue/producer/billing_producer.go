// Package producer contains RabbitMQ producers
package producer

import (
	"context"
	"encoding/json"

	"mikmongo/pkg/rabbitmq"
)

// BillingProducer handles billing-related messages
type BillingProducer struct {
	client *rabbitmq.Client
}

// GenerateInvoiceEvent represents a generate invoice event
type GenerateInvoiceEvent struct {
	CustomerID string  `json:"customer_id"`
	PackageID  string  `json:"package_id"`
	Amount     float64 `json:"amount"`
	Period     string  `json:"period"`
}

// NewBillingProducer creates a new billing producer
func NewBillingProducer(client *rabbitmq.Client) *BillingProducer {
	return &BillingProducer{client: client}
}

// PublishGenerateInvoice publishes a generate invoice event
func (p *BillingProducer) PublishGenerateInvoice(ctx context.Context, event *GenerateInvoiceEvent) error {
	if p.client == nil {
		return errRabbitMQNotConfigured
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, "billing.exchange", "invoice.generate", body)
}
