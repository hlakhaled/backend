package iot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iotdataplane"
)

type Publisher struct {
	Client *iotdataplane.Client
}

func NewPublisher(cfg aws.Config) *Publisher {
	return &Publisher{
		Client: iotdataplane.NewFromConfig(cfg),
	}
}

func (publisher *Publisher) Publish(ctx context.Context, topic string, payload interface{}) error {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload for IoT: %w", err)
	}

	_, err = publisher.Client.Publish(ctx, &iotdataplane.PublishInput{
		Topic:   aws.String(topic),
		Payload: payloadData,
		Qos:     1, 
	})

	if err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", topic, err)
	}

	return nil
}