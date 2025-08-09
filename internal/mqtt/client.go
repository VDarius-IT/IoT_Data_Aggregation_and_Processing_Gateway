package mqtt

import (
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/your-username/iot-edge-gateway/internal/buffer"
	"github.com/your-username/iot-edge-gateway/internal/metrics"
)

type Client struct {
	client paho.Client
	store  *buffer.Store
	topic  string
	qos    byte
}

// New creates and connects an MQTT client and subscribes to the given topic.
// broker - e.g. tcp://localhost:1883
func New(broker, clientID, topic string, qos byte, store *buffer.Store) (*Client, error) {
	if broker == "" {
		return nil, fmt.Errorf("broker required")
	}
	opts := paho.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	c := paho.NewClient(opts)

	token := c.Connect()
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	mc := &Client{
		client: c,
		store:  store,
		topic:  topic,
		qos:    qos,
	}

	// Subscribe with message handler
	if token := c.Subscribe(topic, qos, mc.messageHandler); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return mc, nil
}

func (c *Client) messageHandler(client paho.Client, msg paho.Message) {
	// Enqueue payload to disk-backed buffer; errors are logged to stdout for now.
	if c.store == nil {
		fmt.Println("buffer store is nil; dropping message")
		return
	}
	id, err := c.store.Enqueue(msg.Payload())
	if err != nil {
		fmt.Printf("failed to enqueue message: %v\n", err)
	} else {
		// increment Prometheus counter and update pending gauge
		metrics.Enqueued.Inc()
		if cnt, err := c.store.CountUnsent(); err == nil {
			metrics.BufferPending.Set(float64(cnt))
		}
		// minimal ack/log
		fmt.Printf("enqueued message id=%d from topic %s (len=%d)\n", id, msg.Topic(), len(msg.Payload()))
	}
}

func (c *Client) Close() {
	if c == nil || c.client == nil {
		return
	}
	_ = c.client.Unsubscribe(c.topic)
	c.client.Disconnect(250)
}
