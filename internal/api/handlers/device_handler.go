package handlers

import (
    "log/slog"
    "net/http"
    "time"
    "fmt"
    "context"
	

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

/*func addTempStats(response gin.H, data []models.Telemetry, metric, deviceID string, now int64) {
    stats, err := telemetry.CalculateTempState(data, metric, now)
    if err != nil {
        slog.Warn("failed to calculate temp stats", "device_id", deviceID, "metric", metric, "error", err)
    }
    response["min"] = stats.Min
    response["max"] = stats.Max
    response["average"] = stats.Average
}*/

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

// showing last 5 Recent Events with its time - the Last Activity time - warning and alerts based on unlock time
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
		} else if minutesUnlocked > 7 && minutesUnlocked <= 15 {
			alertStatus = "WARNING"
		}
		payload["security_alert"] = alertStatus
	} else {
		payload["security_alert"] = "SAFE"
	}
}

// getting normal state in door insights
func addDoorInsights(response gin.H, data []models.Telemetry, state *models.DeviceState, now int64) {
	avgUnlock := telemetry.CalculateAvgUnlock(data, now)
	response["average_unlock_minutes"] = avgUnlock

	
	normalDuration := 15.0 
	if userPref, ok := state.Payload["normal_unlock_duration"].(float64); 
    ok {
		normalDuration = userPref
	}
	
	if avgUnlock > normalDuration {
		response["unlock_duration_status"] = "Above Normal"
	} else {
		response["unlock_duration_status"] = "Normal"
	}
}

//getting info for AC based on temp and timer
func (handler *DeviceHandler) showACStats(ctx context.Context, payload map[string]interface{}, now int64) {
	
	insideTemp := 0.0

	tempState, err := handler.StateStore.GetStateByID(ctx, "temp-sensor-01")  //temp sensor name may be changed
	if err == nil && tempState != nil {
		if val, ok := tempState.Payload["temp"].(float64);
        ok {
			insideTemp = val
		}
	}
	payload["inside_temp"] = insideTemp
	
	payload["outside_temp"] = 36.0 // demo for now, api fetch later

	// calculate remaining timer time in manual mode
	if timeremaining, ok := payload["timer_end_timestamp"].(float64); 
    ok {
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

	// calculating ac run time
	if powerState, ok := payload["power_state"].(string); 
    ok && powerState == "ON" {
		if lastOnFloat, ok := payload["last_turned_on"].(float64); 
        ok {
			lastOn := int64(lastOnFloat)
			payload["running_time"] = telemetry.FormatACTime(now - lastOn)
		} else {
			payload["running_time"] = "Unknown"
		}
	} else {
		payload["running_time"] = "Off"
	}
}



// handling GET /devices/:id
func (handler *DeviceHandler) GetDeviceByID(context *gin.Context) {
    deviceID := context.Param("id")

    state, err := handler.StateStore.GetStateByID(context.Request.Context(), deviceID)
    if err != nil {
        context.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }

    if state == nil {
        context.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
        return
    }

    state.Status = devices.ConnectionStatus(state.LastSeenAt)
    if state.Type == "light-sensor" {
        addLightStatus(state.Payload, state.OperationalState)
    }
    if state.Type == "door-actuator" {
		now := time.Now().Unix()
		//get the 5 most recent events
		recentHistory, dbErr := handler.TelemetryStore.GetTelemetryHistory(context.Request.Context(), deviceID, 5, 0)
		if dbErr != nil {
			slog.Warn("failed to fetch recent door history", "device_id", deviceID, "error", dbErr)
		}
		showDoorStats(state.Payload, recentHistory, now)
	}
    if state.Type == "ac-actuator" {
		now := time.Now().Unix()
		recentHistory, dbErr := handler.TelemetryStore.GetTelemetryHistory(context.Request.Context(), deviceID, 5, 0)
		if dbErr != nil {
			slog.Warn("failed to fetch recent AC history", "device_id", deviceID, "error", dbErr)
		} else if len(recentHistory) > 0 {
			state.Payload["recent_events"] = telemetry.FormatACEvents(recentHistory)
		}
	
		handler.showACStats(context.Request.Context(), state.Payload, now)
	}
	if state.Type == "temp-sensor" {
		now := time.Now().Unix()
		cutoff24h := now - 86400
		recentHistory, dbErr := handler.TelemetryStore.GetTelemetryHistory(context.Request.Context(), deviceID, 0, cutoff24h)
		if dbErr != nil {
			slog.Warn("failed to fetch recent temp history", "device_id", deviceID, "error", dbErr)
		} else {
			stats, _ := telemetry.CalculateTempState(recentHistory, "temp", now)
			state.Payload["Min"] = stats.Min
			state.Payload["Max"] = stats.Max
			state.Payload["Average"] = stats.Average
		}
	}

	if state.Type == "gas-sensor" {
		recentHistory, dbErr := handler.TelemetryStore.GetTelemetryHistory(context.Request.Context(), deviceID, 5, 0)
		if dbErr != nil {
			slog.Warn("failed to fetch recent gas history", "device_id", deviceID, "error", dbErr)
		} else if len(recentHistory) > 0 {
			state.Payload["recent_events"] = recentHistory
		}
	}
	

    context.JSON(http.StatusOK, state)
}

// handling GET /devices/:id/telemetry?period=...&metric=...
func (handler *DeviceHandler) GetDeviceTelemetry(context *gin.Context) {
    deviceID := context.Param("id")
    period := context.DefaultQuery("period", "24h")
    metric := context.DefaultQuery("metric", "temp")

    now := time.Now().Unix()
    state, err := handler.StateStore.GetStateByID(context.Request.Context(), deviceID)
    if err != nil {
        context.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    if state == nil {
        context.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
        return
    }

    response := gin.H{
        "device_id": deviceID,
        "period":    period,
    }

    if isHotTier(period) {
        // Pass the period cutoff to DynamoDB 
        cutoff := telemetry.PeriodCutoff(now, period)
        rawData, dbErr := handler.TelemetryStore.GetTelemetryHistory(context.Request.Context(), deviceID, 0, cutoff)
        if dbErr != nil {
            context.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to fetch telemetry history: device_id=%s, period=%s, error=%v", deviceID, period, dbErr),
            })
            return
        }

        response["source"] = "DynamoDB"
        response["data"] = telemetry.FilterTime(rawData, metric, period, now)

       
        if state.Type == "door-actuator" {
			addDoorInsights(response, rawData, state, now)
		}
        if state.Type == "ac-actuator" {
			if period == "7d" { 
				// The Usage Bar Chart
				response["usage_bar"] = telemetry.CalculateACUsage(rawData, now, period)
			}
			
			if period == "24h" {
				totalSeconds := telemetry.CalculateACRunTime(rawData, now)
				response["running_time"] = telemetry.FormatACTime(totalSeconds)
			}
		}

    } else {
        response["source"] = "S3 processed data"
		if period == "1m" {
            currentMonth := time.Now().Format("2006-01")
            s3Key := fmt.Sprintf("processed-charts/%s/%s.json", deviceID, currentMonth)
            s3Data, err := handler.S3Fetcher.GetMonthlyChart(context.Request.Context(), s3Key)  //download json file from s3        
            if err != nil {
                slog.Warn("failed to fetch monthly S3 chart", "device_id", deviceID, "error", err)
                response["data"] = []telemetry.ChartPoint{} //to not cause app crash return an empty array
            } else {
                response["data"] = s3Data // The pre-calculated array from Python!
            }
        } else {
             response["data"] = []telemetry.ChartPoint{}
        }
       
}

    context.JSON(http.StatusOK, response)
}

func (handler *DeviceHandler) GetDeviceAlerts(context *gin.Context) {
    deviceID := context.Param("id")

    state, err := handler.StateStore.GetStateByID(context.Request.Context(), deviceID)
    if err != nil {
        context.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    if state == nil {
        context.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
        return
    }

    alertList, err := handler.AlertStore.GetAlertsByDevice(context.Request.Context(), deviceID, 0)
    if err != nil {
        context.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch alerts"})
        return
    }
    context.JSON(http.StatusOK, gin.H{"data": alertList})
}
func isHotTier(period string) bool {
    switch period {
    case  "24h", "7d":
        return true
    default:
        return false
    }
}

//handling GET /system/overview
func (handler *DeviceHandler) GetSystemOverview(context *gin.Context) {
	timeFilter := context.DefaultQuery("period", "7d") // default -> 7d
	now := time.Now().Unix()
	cutoff := telemetry.PeriodCutoff(now, timeFilter)

	
	states, err := handler.StateStore.GetAllStates(context.Request.Context())
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch device states"})
		return
	}

	onlineCount := 0
	for _, state := range states {   //count how many online devices
		if devices.ConnectionStatus(state.LastSeenAt) == "ONLINE" {
			onlineCount++
		}
	}

	systemStatus := "Offline"
	if onlineCount > 0 {
		systemStatus = "Connected"
	}
    //get alerts
	alertsList, err := handler.AlertStore.GetAllAlerts(context.Request.Context(), cutoff)
	if err != nil {
		slog.Warn("Failed to get alerts for system overview", "error", err)
	}
	alertsChart := telemetry.GetAlerts(alertsList, timeFilter)

    //calculate Energy Consumption
	acData, err := handler.TelemetryStore.GetTelemetryHistory(context.Request.Context(), "ac-01", 0, cutoff)  //ac name may be changed later
	if err != nil {
		slog.Warn("Failed to fetch AC telemetry for energy chart", "error", err)
	}
	
	acUsage := telemetry.CalculateACUsage(acData, now, timeFilter)
	energyData := telemetry.CalculateEnergy(acUsage)


	context.JSON(http.StatusOK, gin.H{
		"system_status":  systemStatus,
		"devices_online": fmt.Sprintf("%d / %d", onlineCount, len(states)),
		"alerts_chart":   alertsChart,
        "energy_consumption": energyData,
	})
}



//handling POST /devices/:id/commands
func (handler *DeviceHandler) SendCommand(context *gin.Context) {
	deviceID := context.Param("id")

	var req SendCommandRequest
	if err := context.ShouldBindJSON(&req); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid command format! action is required."})
		return
	}

	requestID := fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	mqttPayload := map[string]interface{}{
		"request_id": requestID,
		"action":     req.Action,
		"parameters": req.Parameters,
	}

	topic := fmt.Sprintf("devices/%s/command", deviceID)
	err := handler.IoTPublisher.Publish(context.Request.Context(), topic, mqttPayload)
	if err != nil {
		slog.Error("failed to publish command to iot Core", "device_id", deviceID, "error", err)
		context.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to communicate with device"})
		return
	}

	commandRecord := models.Command{
		RequestID:  requestID,
		DeviceID:   deviceID,
		Timestamp:  time.Now().Unix(),
		Action:     req.Action,
		Parameters: req.Parameters,
	}
	
	if storeErr := handler.CommandStore.SaveCommand(context.Request.Context(), commandRecord); storeErr != nil {
		slog.Warn("Command sent, but failed to save history to DB", "error", storeErr)
	}

	context.JSON(http.StatusAccepted, gin.H{
		"message":    "Command dispatched successfully",
		"request_id": requestID,
	})
}