package models

type Command struct {
	RequestID string                 `json:"request_id" dynamodbav:"request_id"`
	DeviceID  string                 `json:"device_id" dynamodbav:"device_id"`
	Timestamp int64                  `json:"timestamp" dynamodbav:"timestamp"`
	Action    string                 `json:"action" dynamodbav:"action"`
	Parameters map[string]interface{} `json:"parameters" dynamodbav:"parameters"`
	ExpiresAt int64                  `json:"expires_at" dynamodbav:"expires_at"`
}
