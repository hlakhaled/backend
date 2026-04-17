package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Fleexa-Graduation-Project/Backend/internal/alerts"
	"github.com/Fleexa-Graduation-Project/Backend/internal/devices"
	"github.com/Fleexa-Graduation-Project/Backend/internal/telemetry"
	"github.com/Fleexa-Graduation-Project/Backend/internal/validation"
	"github.com/Fleexa-Graduation-Project/Backend/models"
)

type Service struct {
	Logger         *slog.Logger
	TelemetryStore *telemetry.TelemetryStore
	AlertStore     *alerts.AlertStore
	StateStore     *devices.StateStore
}

func (s *Service) HandleRequest(ctx context.Context, event map[string]interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			s.Logger.Error("CRITICAL: Lambda Panic Recovered", "panic", r)
			err = fmt.Errorf("internal server error")
		}
	}() 

	//validating the message
	deviceID, messageType, envelope, isBatch, err := validation.ValidateMessage(event)
	if err != nil {
		s.logValidationError(err, envelope.DeviceID)
		return err
	}
	
	switch messageType {
	case "telemetry":
		return s.handleTelemetry(ctx, deviceID, envelope, isBatch)

	case "alerts":
		return s.handleAlert(ctx, deviceID, envelope)

	default:
		return fmt.Errorf("unknown message type: %s", messageType)
	}
}

func (service *Service) handleTelemetry(ctx context.Context, deviceID string, envelope models.MQTTEnvelope, isBatch bool) error {

	if isBatch {
		items, ok := envelope.Payload["items"].([]interface{})
		if !ok {
			return fmt.Errorf("invalid batch format: items is not a list")
		}

		service.Logger.Info("processing batch telemetry", "device_id", deviceID, "count", len(items))

		var telemetryList []models.Telemetry
		var latestReading models.Telemetry

		for _, itemRaw := range items {
			itemMap, ok := itemRaw.(map[string]interface{})
			if !ok {
				service.Logger.Warn("Skipping invalid item in batch", "device_id", deviceID)
				continue
			}

			// Validating individual item structure
			if err := validation.ValidatePayload(envelope.Type, itemMap); err != nil {
				service.Logger.Warn("Skipping malformed payload in batch", "error", err)
				continue
			}

			timestamp := envelope.Timestamp
			if itemTs, ok := itemMap["ts"].(float64); ok {
				timestamp = int64(itemTs)
			}

			t := models.Telemetry{
				DeviceID:  deviceID,
				Timestamp: timestamp,
				Type:      envelope.Type,
				Payload:   itemMap,
			}

			telemetryList = append(telemetryList, t)

			//select the latest timestamp
			if latestReading.Timestamp == 0 || t.Timestamp > latestReading.Timestamp {
				latestReading = t
			}
		}

		if len(telemetryList) > 0 {
			if err := service.TelemetryStore.SaveTelemetryBatch(ctx, telemetryList); err != nil {
				service.Logger.Error("Failed to save batch telemetry", "error", err)
				return err
			}

			return service.StateStore.UpdateFromTelemetry(ctx, latestReading)
		}
		return nil
	}

	data := models.Telemetry{
		DeviceID:  envelope.DeviceID,
		Timestamp: envelope.Timestamp,
		Type:      envelope.Type,
		Payload:   envelope.Payload,
	}

	service.Logger.Info("saving single telemetry", "device_id", deviceID)

	if err := service.TelemetryStore.SaveTelemetry(ctx, data); err != nil {
		service.Logger.Error("failed to save telemetry", "error", err)
		return err
	}

	return service.StateStore.UpdateFromTelemetry(ctx, data)
}

func (service *Service) handleAlert(ctx context.Context, deviceID string, envelope models.MQTTEnvelope) error {
	severity, _ := envelope.Payload["severity"].(string)

	alert := models.Alert{
		DeviceID:  deviceID,
		Timestamp: envelope.Timestamp,
		Type:      envelope.Type,
		Severity:  severity,
		Payload:   envelope.Payload,
	}

	if err := service.AlertStore.SaveAlert(ctx, alert); err != nil {
		return err
	}

	return service.StateStore.UpdateHeartbeat(ctx, deviceID)
}

func (service *Service) logValidationError(err error, deviceID string) {
	switch {
	case errors.Is(err, validation.ErrInvalidEvent), errors.Is(err, validation.ErrInvalidEnvelope):
		service.Logger.Warn("invalid message envelope", "error", err)
	case errors.Is(err, validation.ErrInvalidPayload):
		service.Logger.Warn("invalid payload", "device_id", deviceID, "error", err)
	case errors.Is(err, validation.ErrInvalidTopic):
		service.Logger.Warn("invalid topic", "error", err)
	default:
		service.Logger.Error("unexpected validation error", "error", err)
	}
}
