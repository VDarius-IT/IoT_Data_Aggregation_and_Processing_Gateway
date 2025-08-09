package kafka

import (
	"fmt"
	"sync"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/kafka"
)

// ProducerClient is the minimal interface used by the forwarder to send messages.
// This allows providing a mock implementation for tests.
type ProducerClient interface {
	Produce([]byte, time.Duration) error
	Close()
}

type Producer struct {
	p     *confluent.Producer
	topic string
	wg    sync.WaitGroup
	closed bool
}

// NewProducer creates a confluent Kafka producer.
// brokers: comma-separated broker list, topic is target topic, clientID optional.
func NewProducer(brokers string, topic string, clientID string) (*Producer, error) {
	if brokers == "" {
		return nil, fmt.Errorf("brokers required")
	}
	cfg := &confluent.ConfigMap{
		"bootstrap.servers": brokers,
		"acks": "all",
	}
	if clientID != "" {
		_ = cfg.SetKey("client.id", clientID)
	}
	p, err := confluent.NewProducer(cfg)
	if err != nil {
		return nil, err
	}
	pr := &Producer{
		p: p,
		topic: topic,
	}
	// Start background delivery handler
	pr.wg.Add(1)
	go pr.deliveryHandler()
	return pr, nil
}

func (pr *Producer) deliveryHandler() {
	defer pr.wg.Done()
	for e := range pr.p.Events() {
		switch ev := e.(type) {
		case *confluent.Message:
			if ev.TopicPartition.Error != nil {
				// delivery failed
				// For now we log via fmt; higher-level code should handle retries
				fmt.Printf("Delivery failed: %v\n", ev.TopicPartition)
			} else {
				fmt.Printf("Delivered message to %v\n", ev.TopicPartition)
			}
		default:
			// ignore other events
		}
	}
}

// Produce sends a message and waits up to timeout for delivery report.
// Returns nil on success.
func (pr *Producer) Produce(payload []byte, timeout time.Duration) error {
	if pr == nil || pr.p == nil {
		return fmt.Errorf("producer not initialized")
	}
	if pr.closed {
		return fmt.Errorf("producer closed")
	}
	msg := &confluent.Message{
		TopicPartition: confluent.TopicPartition{Topic: &pr.topic, Partition: confluent.PartitionAny},
		Value: payload,
	}
	// Produce with delivery channel
	deliveryChan := make(chan confluent.Event, 1)
	err := pr.p.Produce(msg, deliveryChan)
	if err != nil {
		close(deliveryChan)
		return err
	}
	select {
	case ev := <-deliveryChan:
		m, ok := ev.(*confluent.Message)
		if !ok {
			return fmt.Errorf("unexpected delivery event type")
		}
		if m.TopicPartition.Error != nil {
			return m.TopicPartition.Error
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("delivery timeout")
	}
}

// Close flushes and closes the producer.
func (pr *Producer) Close() {
	if pr == nil || pr.p == nil {
		return
	}
	// Give up to 5s to flush
	pr.p.Flush(5000)
	pr.p.Close()
	pr.closed = true
	// Wait for handler to exit
	pr.wg.Wait()
}
