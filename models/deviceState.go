package models

type DeviceState struct {
	DeviceID         string                 `json:"device_id" dynamodbav:"device_id"`
	Type             string                 `json:"type" dynamodbav:"type"`
	Status           string                 `json:"status" dynamodbav:"status"` // online - offline 
	OperationalState string                 `json:"operational_state" dynamodbav:"operational_state"` // based on device: LOCKED-HOT-BRIGHT-OFF etc.
	Health           string                 `json:"health" dynamodbav:"health"`
	Payload          map[string]interface{} `json:"payload" dynamodbav:"payload"` // Raw sensor data (temp, gas_level)
	LastSeenAt       int64                  `json:"last_seen_at" dynamodbav:"last_seen_at"`
	LastUpdated      int64                  `json:"-" dynamodbav:"updated_at"` 
}