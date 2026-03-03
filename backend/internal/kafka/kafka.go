package kafka

import (
	"context"
	"fmt"
	"log"

	"minion-bank-backend/internal/config"

	"github.com/segmentio/kafka-go"
)

var Writer *kafka.Writer

func InitKafka(cfg *config.Config) {
	createTopic(cfg)

	Writer = &kafka.Writer{
		Addr:     kafka.TCP(cfg.KafkaBrokers),
		Topic:    "card-requests",
		Balancer: &kafka.LeastBytes{},
	}
}

func createTopic(cfg *config.Config) {
	conn, err := kafka.Dial("tcp", cfg.KafkaBrokers)
	if err != nil {
		log.Printf("Connecting to Kafka failed: %v", err)
		return
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		log.Printf("Getting Kafka controller failed: %v", err)
		return
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		log.Printf("Connecting to controller failed: %v", err)
		return
	}
	defer controllerConn.Close()

	topicConfig := []kafka.TopicConfig{
		{
			Topic:             "card-requests",
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}

	if err := controllerConn.CreateTopics(topicConfig...); err != nil {
		log.Printf("Kafka topic creation failed (maybe exists): %v", err)
	} else {
		log.Println("Kafka topic 'card-requests' ensured")
	}
}

func PublishMessage(msg []byte) error {
	return Writer.WriteMessages(context.Background(), kafka.Message{
		Value: msg,
	})
}
