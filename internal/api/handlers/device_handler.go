package handlers

import (
    "log/slog"
    "net/http"
    "time"
    "fmt"

    "github.com/Fleexa-Graduation-Project/Backend/internal/devices"
    "github.com/Fleexa-Graduation-Project/Backend/internal/telemetry"
    "github.com/Fleexa-Graduation-Project/Backend/models"
    "github.com/Fleexa-Graduation-Project/Backend/internal/alerts"
    "github.com/Fleexa-Graduation-Project/Backend/internal/commands"
    "github.com/Fleexa-Graduation-Project/Backend/internal/iot"
    "github.com/gin-gonic/gin"
)

type DeviceHandler struct {
    StateStore     *devices.StateStore
    TelemetryStore *telemetry.TelemetryStore
    AlertStore     *alerts.AlertStore
    CommandStore   *commands.CommandStore 
    IoTPublisher   *iot.Publisher
    S3Fetcher      *iot.S3Client
}

type SendCommandRequest struct {
    Action     string                 `json:"action" binding:"required"`
    Parameters map[string]interface{} `json:"parameters"`
}

func addLightStatus(payload map[string]interface{}, operationalState string) {
    switch operationalState {
    case "BRIGHT":
        payload["light_status"] = "Bright"
    case "DARK":
        payload["light_status"] = "Dark"
    case "NORMAL":
        payload["light_status"] = "Normal"
    }
}

// handling GET /devices
func (handler *DeviceHandler) GetDevices(context *gin.Context) {
    states, err := handler.StateStore.GetAllStates(context.Request.Context())
    if err != nil {
        context.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch device states"})
        return
    }
    for i := range states {
        states[i].Status = devices.ConnectionStatus(states[i].LastSeenAt)
        if states[i].Type == "light-sensor" {
            addLightStatus(states[i].Payload, states[i].OperationalState)
        }
    }
    context.JSON(http.StatusOK, gin.H{"data": states})
}

func showDoorStats(payload map[string]interface{}, history []models.Telemetry, now int64) {
    if len(history) == 0 {
        payload["recent_events"] = []map[string]interface{}{}
        payload["last_activity_time"] = "No activity"
        payload["security_alert"] = "SAFE"
        return
    }
    payload["recent_events"] = telemetry.FormatDoorEvents(history)
    payload["last_activity_time"] = telemetry.TimeAgo(history[0].Timestamp, now)

    if lockState, ok := payload["lock_state"].(string); ok && lockState == "UNLOCKED" {
        minutesUnlocked := float64(now-history[0].Timestamp) / 60.0

        alertStatus := "SAFE"
        if minutesUnlocked > 15 {
            alertStatus = "CRITICAL_ALERT"
        } else if minutesUnlocked > 7 {
            alertStatus = "WARNING"
        }
        payload["security_alert"] = alertStatus
    } else {
        payload["security_alert"] = "SAFE"
    }
}

func addDoorInsights(response gin.H, data []models.Telemetry, state *models.DeviceState, now int64) {
    avgUnlock := telemetry.CalculateAvgUnlock(data, now)
    response["average_unlock_minutes"] = avgUnlock

    normalDuration := 15.0
    if userPref, ok := state.Payload["normal_unlock_duration"].(float64); ok {
        normalDuration = userPref
    }

    if avgUnlock > normalDuration {
        response["unlock_duration_status"] = "Above Normal"
    } else {
        response["unlock_duration_status"] = "Normal"
    }
}

func (handler *DeviceHandler) showACStats(ctx interface{}, payload map[string]interface{}, now int64) {
    insideTemp := 0.0

    tempState, err := handler.StateStore.GetStateByID(ctx.(interface{ Done() <-chan struct{} }), "temp-sensor-01")
    if err == nil && tempState != nil {
        if val, ok := tempState.Payload["temp"].(float64); ok {
            insideTemp = val
        }
    }
    payload["inside_temp"] = insideTemp
    payload["outside_temp"] = 36.0

    if timeremaining, ok := payload["timer_end_timestamp"].(float64); ok {
        timerEnd := int64(timeremaining)
        if timerEnd == 0 {
            payload["time_remaining"] = "No active timer"
        } else if timerEnd > now {
            payload["time_remaining"] = telemetry.FormatACTime(timerEnd - now)
        } else {
            payload["time_remaining"] = "Ended"
        }
    } else {
        payload["time_remaining"] = "No active timer"
    }

    if powerState, ok := payload["power_state"].(string); ok && powerState == "ON" {
        if lastOnFloat, ok := payload["last_turned_on"].(float64); ok {
            lastOn := int64(lastOnFloat)
            payload["running_time"] = telemetry.FormatACTime(now - lastOn)
        } else {
            payload["running_time"] = "Unknown"
        }
    } else {
        payload["running_time"] = "Off"
    }
}

// handling GET /devices/:id/telemetry
func (handler *DeviceHandler) GetDeviceTelemetry(context *gin.Context) {
    deviceID := context.Param("id")
    period := context.DefaultQuery("period", "24h")
    metric := context.DefaultQuery("metric", "temp")

    now := time.Now().Unix()

    response := gin.H{
        "device_id": deviceID,
        "period":    period,
    }

    if !isHotTier(period) {
        response["source"] = "S3 processed data"

        if period == "1m" {
            currentMonth := time.Now().Format("2006-01")
            s3Key := fmt.Sprintf("processed-charts/%s/%s.json", deviceID, currentMonth)

            // ✅ SAFE FIX
            if handler.S3Fetcher == nil {
                response["data"] = []telemetry.ChartPoint{}
            } else {
                s3Data, err := handler.S3Fetcher.GetMonthlyChart(context.Request.Context(), s3Key)
                if err != nil {
                    slog.Warn("failed to fetch monthly S3 chart", "device_id", deviceID, "error", err)
                    response["data"] = []telemetry.ChartPoint{}
                } else {
                    response["data"] = s3Data
                }
            }
        } else {
            response["data"] = []telemetry.ChartPoint{}
        }
    }

    context.JSON(http.StatusOK, response)
}

func isHotTier(period string) bool {
    switch period {
    case "24h", "7d":
        return true
    default:
        return false
    }
}