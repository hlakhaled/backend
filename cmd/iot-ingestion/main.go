package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/Fleexa-Graduation-Project/Backend/internal/ingestion"
	"github.com/Fleexa-Graduation-Project/Backend/internal/telemetry"
	"github.com/Fleexa-Graduation-Project/Backend/internal/alerts"
	"github.com/Fleexa-Graduation-Project/Backend/internal/devices"
	"github.com/Fleexa-Graduation-Project/Backend/pkg/db"
	"github.com/Fleexa-Graduation-Project/Backend/pkg/logger"
)

var (
	log            *slog.Logger
	telemetryStore *telemetry.TelemetryStore
	alertStore     *alerts.AlertStore
	stateStore     *devices.StateStore
)

func init() {

	log = logger.InitLogger()
	log.Info("lambda function-> cold Start...")

	if err := db.NewDynamoDBClient(context.Background()); err != nil {
		log.Error("failed to initialize DynamoDB", "error", err)
		panic(err)
	}

	var err error

	telemetryStore, err = telemetry.NewTelemetryStore()
	if err != nil {
		panic(fmt.Errorf("failed to init telemetry store: %w", err))
	}

	alertStore, err = alerts.NewAlertStore()
	if err != nil {
		panic(fmt.Errorf("failed to init alert store: %w", err))
	}

	stateStore, err = devices.NewStateStore()
	if err != nil {
		panic(fmt.Errorf("failed to init device state store: %w", err))
	}

	log.Info("iot ingestion -> Cold Start Completed. Stores Ready.")

}

func main() {

	service := &ingestion.Service{
		Logger:         log,
		TelemetryStore: telemetryStore,
		AlertStore:     alertStore,
		StateStore:     stateStore,
	}

	lambda.Start(service.HandleRequest)
}