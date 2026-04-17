# Fleexa API Specification (v1)

**Base URL:** `http://localhost:8080/api/v1`  
**Content-Type:** `application/json`

---

## 1. System Overview & Device State

### 1.1 Get Global System Overview (NEW)

Retrieves high-level aggregated data for the Global Dashboard (system health, alerts, energy).

- **Endpoint:** `GET /system/overview`
- **Query Parameters (Optional):**
  - `period` (string): `24h`, `7d`, `1m`

- **Response (200 OK):**
```json
{
  "system_status": "Connected",
  "devices_online": "5 / 5",
  "alerts_chart": {
    "critical": [
      { "label": "Mon", "value": 0 },
      { "label": "Tue", "value": 2 }
    ],
    "warning": [
      { "label": "Mon", "value": 3 },
      { "label": "Tue", "value": 1 }
    ]
  },
  "energy_consumption": [
    { "label": "Mon", "value": 12.4 },
    { "label": "Tue", "value": 15.1 }
  ]
}
```

---

### 1.2 Get All Devices List

Retrieves live status of all devices.

- **Endpoint:** `GET /devices`

- **Response (200 OK):**
```json
{
  "data": [
    {
      "device_id": "temp-sensor-01",
      "type": "temp-sensor",
      "status": "ONLINE",
      "operational_state": "NORMAL",
      "health": "HEALTHY",
      "payload": {
        "temp": 24.5
      },
      "last_seen_at": 1708434000
    },
    {
      "device_id": "ac-actuator-01",
      "type": "ac-actuator",
      "status": "ONLINE",
      "operational_state": "ON",
      "health": "HEALTHY",
      "payload": {
        "power_state": "ON",
        "target_temp": 24.0,
        "mode": "COOLING",
        "last_turned_on": 1708434000,
        "timer_end_timestamp": 0
      },
      "last_seen_at": 1708434000
    }
  ]
}
```

---

### 1.3 Get Specific Device Details

Retrieves full device state + insights.

- **Endpoint:** `GET /devices/:id`

- **Response (200 OK):**
```json
{
  "device_id": "ac-actuator-01",
  "type": "ac-actuator",
  "status": "ONLINE",
  "operational_state": "ON",
  "health": "HEALTHY",
  "payload": {
    "power_state": "ON",
    "target_temp": 24.0,
    "mode": "COOLING",
    "last_turned_on": 1708434000,
    "timer_end_timestamp": 1708437600,
    "inside_temp": 25.5,
    "outside_temp": 36.0,
    "time_remaining": "1h 0m",
    "running_time": "2h 30m",
    "recent_events": [
      {
        "event": "A/C turned ON",
        "time": "3:04 PM",
        "timestamp": 1708434000
      }
    ]
  },
  "last_seen_at": 1708434000
}
```

---

## 2. Telemetry, Analytics, and Alerts (The Insights)

### 2.1 Get Device Telemetry & Insights

Retrieves historical data + analytics.

- **Endpoint:** `GET /devices/:id/telemetry`
- **Query Parameters:**
  - `period`: `24h`, `7d`, `1m`
  - `metric`: e.g. `temp`, `light_level`

- **Response (200 OK):**
```json
{
  "device_id": "temp-sensor-01",
  "period": "24h",
  "source": "DynamoDB",
  "data": [
    { "label": "14:00", "value": 29.0 },
    { "label": "15:00", "value": 28.5 }
  ],
  "min": 22.0,
  "max": 29.0,
  "average": 25.4
}
```

---

### 2.2 Get Device Alerts

Retrieves warnings & critical events.

- **Endpoint:** `GET /devices/:id/alerts`

- **Response (200 OK):**
```json
{
  "data": [
    {
      "device_id": "gas-sensor-01",
      "timestamp": 1708430000,
      "type": "gas-sensor",
      "severity": "CRITICAL",
      "payload": {
        "gas_level": 950,
        "status": "DANGER",
        "alarm_on": true
      }
    }
  ]
}
```

---

## 3. Device Control (Actuators)

### 3.1 Send Command to Device

Sends command to actuator via MQTT.

- **Endpoint:** `POST /devices/:id/commands`

- **Request Body:**
```json
{
  "action": "SET_STATE",
  "parameters": {
    "power_state": "ON",
    "target_temp": 24.0,
    "mode": "COOLING"
  }
}
```

- **Response (202 Accepted):**
```json
{
  "message": "Command dispatched successfully",
  "request_id": "cmd-1708434000123"
}
```

---

## 4. Authentication & Security (Upcoming)

Authentication will be handled via AWS Cognito or a dedicated service.

### 4.1 Planned Auth Flows

- **Sign In:** `POST /auth/login` → Returns JWT  
- **Sign Up:** `POST /auth/register`  
- **Verify:** `POST /auth/verify`