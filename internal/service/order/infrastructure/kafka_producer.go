package infrastructure

import (
	"context"
	"encoding/json"
	"log"
	"nexus/internal/pkg/mq"
	"nexus/internal/service/order/domain"

	"github.com/segmentio/kafka-go"
)

type OrderProducerAdapter struct {
	writer *kafka.Writer
}

func NewOrderProducerAdapter(writer *kafka.Writer) *OrderProducerAdapter {
	return &OrderProducerAdapter{writer: writer}
}

func (p *OrderProducerAdapter) Product(ctx context.Context, event *domain.OrderCreationRequested) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("ERROR: Failed to marshal order creation event: %v", err)
		return err
	}

	err = mq.ProduceMessage(ctx, p.writer, []byte(event.UserID), eventBytes)
	if err != nil {
		log.Printf("ERROR: Failed to produce message to Kafka: %v", err)
		return err
	}
	return nil
}
