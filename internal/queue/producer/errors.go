package producer

import "errors"

var errRabbitMQNotConfigured = errors.New("rabbitmq client is not configured")
