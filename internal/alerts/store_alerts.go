
package alerts

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/Fleexa-Graduation-Project/Backend/models"
	"github.com/Fleexa-Graduation-Project/Backend/pkg/db"
)

type AlertStore struct {
	Client    *dynamodb.Client
	TableName string
}

func NewAlertStore() (*AlertStore, error) {
	tableName := os.Getenv("DYNAMODB_ALERTS_TABLE")
	if tableName == "" {
		return nil, fmt.Errorf("DYNAMODB_ALERTS_TABLE environment variable is not set")
	}

	if db.Client == nil {
		return nil, fmt.Errorf("dynamodb client is not initialized")
	}

	return &AlertStore{
		Client:    db.Client,
		TableName: tableName,
	}, nil
}

func (store *AlertStore) SaveAlert(ctx context.Context, alert models.Alert) error {
	if alert.ExpiresAt == 0 {
		alert.ExpiresAt = time.Now().Add(30 * 24 * time.Hour).Unix()
	}

	item, err := attributevalue.MarshalMap(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(store.TableName),
		Item:      item,
	}

	_, err = store.Client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to store alert in dynamodb: %w", err)
	}

	return nil
}


func (store *AlertStore) GetAlertsBySeverity(ctx context.Context, severity string, limit int32,
	) ([]models.Alert, error) {

	const defaultLimit int32 = 20
	if limit <= 0 {
		limit = defaultLimit
	}

	input := &dynamodb.QueryInput{
		TableName: aws.String(store.TableName),

		IndexName: aws.String("SeverityIndex"),

		KeyConditionExpression: aws.String("severity = :severity"),  //partition key condition of the GSI.

		ExpressionAttributeValues: map[string]types.AttributeValue{
			":severity": &types.AttributeValueMemberS{
				Value: severity,
			},
		},

		ScanIndexForward: aws.Bool(false), // newest alerts first

		Limit: aws.Int32(limit),
	}

	res, err := store.Client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts by severity: %w", err)
	}

	var alerts []models.Alert
	err = attributevalue.UnmarshalListOfMaps(res.Items, &alerts)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal alerts: %w", err)
	}

	return alerts, nil
}

//return recent alerts for a specific device
func (store *AlertStore) GetAlertsByDevice(ctx context.Context, deviceID string, limit int32) ([]models.Alert, error) {
    const defaultLimit int32 = 20
    if limit <= 0 {
        limit = defaultLimit
    }

    input := &dynamodb.QueryInput{
        TableName:              aws.String(store.TableName),
        KeyConditionExpression: aws.String("device_id = :id"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":id": &types.AttributeValueMemberS{Value: deviceID},
        },
        ScanIndexForward: aws.Bool(false), //newest first
        Limit:            aws.Int32(limit),
    }

    res, err := store.Client.Query(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("failed to query alerts for device %s: %w", deviceID, err)
    }

    var alertList []models.Alert
    if err = attributevalue.UnmarshalListOfMaps(res.Items, &alertList); err != nil {
        return nil, fmt.Errorf("failed to unmarshal alerts for device %s: %w", deviceID, err)
    }

    return alertList, nil
}


//retrieve all alerts in the whole system (system overview part)
func (store *AlertStore) GetAllAlerts(ctx context.Context, since int64) ([]models.Alert, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(store.TableName),
		FilterExpression: aws.String("#ts >= :since"),
		ExpressionAttributeNames: map[string]string{
			"#ts": "timestamp",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":since": &types.AttributeValueMemberN{Value: fmt.Sprint(since)},
		},
	}

	res, err := store.Client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan all alerts: %w", err)
	}

	var alerts []models.Alert
	if err = attributevalue.UnmarshalListOfMaps(res.Items, &alerts);
	  err != nil {
		return nil, fmt.Errorf("failed to unmarshal alerts: %w", err)
	}

	return alerts, nil
}
