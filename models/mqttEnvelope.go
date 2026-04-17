package models

// incoming message structure
type MQTTEnvelope struct {
	DeviceID  string                 `json:"device_id"`
	Timestamp int64                  `json:"timestamp"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
}