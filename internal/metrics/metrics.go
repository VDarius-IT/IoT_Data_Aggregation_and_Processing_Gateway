package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	Enqueued = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "iot_enqueue_total",
		Help: "Total number of messages enqueued to the local buffer",
	})
	Forwarded = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "iot_forwarded_total",
		Help: "Total number of messages successfully forwarded to Kafka",
	})
	ForwardFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "iot_forward_failed_total",
		Help: "Total number of messages that failed forwarding after retries",
	})
	BufferPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iot_buffer_pending",
		Help: "Current number of pending (unsent) messages in the buffer",
	})
)

func Init() {
	prometheus.MustRegister(Enqueued, Forwarded, ForwardFailed, BufferPending)
}
