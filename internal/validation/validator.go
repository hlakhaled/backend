package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Fleexa-Graduation-Project/Backend/internal/devices"
	"github.com/Fleexa-Graduation-Project/Backend/models"
)

var (
	ErrInvalidEvent    = errors.New("invalid event")
	ErrInvalidTopic    = errors.New("invalid topic")
	ErrInvalidEnvelope = errors.New("invalid envelope")
	ErrInvalidPayload  = errors.New("invalid payload")
)

//validating incoming messages
func ValidateMessage(event map[string]interface{}) (
	deviceID string,
	messageType string,
	envelope models.MQTTEnvelope,
	isBatch bool, // <--- New Return Value
	err error,
) {

	//validating raw event
	topic, payloadRaw, err := validateEvent(event)
	if err != nil {
		return "", "", envelope, false, err
	}

	// validating topic
	deviceID, messageType, err = validateTopic(topic)
	if err != nil {
		return "", "", envelope, false, err
	}

	// decoding payload into envelope
	if err := decodeEnvelope(payloadRaw, &envelope); err != nil {
		return "", "", envelope, false, err
	}

	// CHECK FOR BATCH: Look for "items" key in the payload
	if messageType == "telemetry" {
	if items, ok := envelope.Payload["items"].([]interface{}); ok && len(items) > 0 {
		isBatch = true
	}
}

	// validating envelope fields
	if err := validateEnvelope(envelope, deviceID); err != nil {
		return "", "", envelope, false, err
	}

	// validating payload structure
	// If it is a batch, we SKIP deep validation here (we will do it in the loop later)
	if !isBatch {
		if err := ValidatePayload(envelope.Type, envelope.Payload); err != nil {
			return "", "", envelope, false, err
		}

		// alert-specific validation
		if messageType == "alerts" {
			severity, ok := envelope.Payload["severity"].(string)
			if !ok || severity == "" {
				return "", "", envelope, false,
					fmt.Errorf("%w: alert missing or invalid severity", ErrInvalidPayload)
			}
		}
	}

	return deviceID, messageType, envelope, isBatch, nil
}

// --- Helper Functions (No Changes) ---

func validateEvent(event map[string]interface{}) (string, interface{}, error) {
	topic, ok := event["topic"].(string)
	if !ok || topic == "" {
		return "", nil, fmt.Errorf("%w: missing or invalid topic", ErrInvalidEvent)
	}
	payload, ok := event["payload"]
	if !ok {
		return "", nil, fmt.Errorf("%w: missing payload", ErrInvalidEvent)
	}
	return topic, payload, nil
}

func validateTopic(topic string) (string, string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("%w: expected devices/{id}/{type}", ErrInvalidTopic)
	}
	if parts[0] != "devices" {
		return "", "", fmt.Errorf("%w: invalid topic root", ErrInvalidTopic)
	}
	deviceID := parts[1]
	messageType := parts[2]
	if deviceID == "" {
		return "", "", fmt.Errorf("%w: empty device id", ErrInvalidTopic)
	}
	switch messageType {
	case "telemetry", "alerts":
		return deviceID, messageType, nil
	default:
		return "", "", fmt.Errorf("%w: unsupported message type", ErrInvalidTopic)
	}
}

func decodeEnvelope(payloadRaw interface{}, env *models.MQTTEnvelope) error {
	bytes, err := json.Marshal(payloadRaw)
	if err != nil {
		return fmt.Errorf("%w: payload marshal failed", ErrInvalidEnvelope)
	}
	if len(bytes) > 32*1024 {
		return fmt.Errorf("%w: payload too large", ErrInvalidPayload)
	}
	if err := json.Unmarshal(bytes, env); err != nil {
		return fmt.Errorf("%w: payload unmarshal failed", ErrInvalidEnvelope)
	}
	return nil
}

func validateEnvelope(env models.MQTTEnvelope, topicDeviceID string) error {
	if env.DeviceID == "" {
		return fmt.Errorf("%w: missing device_id", ErrInvalidEnvelope)
	}
	if env.DeviceID != topicDeviceID {
		return fmt.Errorf("%w: device_id mismatch", ErrInvalidEnvelope)
	}
	now := time.Now().Unix()
	if env.Timestamp > now+60 {
		return fmt.Errorf("%w: timestamp in the future", ErrInvalidEnvelope)
	}
	if env.Type == "" {
		return fmt.Errorf("%w: missing device type", ErrInvalidEnvelope)
	}
	if len(env.Payload) == 0 {
		return fmt.Errorf("%w: empty payload", ErrInvalidEnvelope)
	}
	return nil
}

func ValidatePayload(deviceType string, payload map[string]interface{}) error {
	rules, ok := devices.Rules[deviceType]
	if !ok {
		return fmt.Errorf("%w: unknown device type", ErrInvalidPayload)
	}
	op := rules.ExtractOperational(payload)
	if op == "UNKNOWN" {
		return fmt.Errorf("%w: payload does not match device type", ErrInvalidPayload)
	}
	switch deviceType {
	case "temp-sensor":
		if _, ok := payload["temp"].(float64); !ok {
			return fmt.Errorf("%w: temp must be numeric", ErrInvalidPayload)
		}
	}
	return nil
}