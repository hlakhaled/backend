package devices

type DeviceRules struct {
	ExtractOperational func(payload map[string]interface{}) string
	EvaluateHealth     func(opState string) string
}
