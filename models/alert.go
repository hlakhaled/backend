package models

type Alert struct {
	DeviceID  string                 `json:"device_id" dynamodbav:"device_id"`
	Timestamp int64                  `json:"timestamp" dynamodbav:"timestamp"`
	Type      string                 `json:"type" dynamodbav:"type"`
	Severity  string                 `json:"severity" dynamodbav:"severity"`
	Payload   map[string]interface{} `json:"payload" dynamodbav:"payload"`
	ExpiresAt int64                  `json:"expires_at" dynamodbav:"expires_at"`
}
