package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Fleexa-Graduation-Project/Backend/models"
	"github.com/Fleexa-Graduation-Project/Backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
    
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	dynamoBatchLimit = 25 // DynamoDB BatchWriteItem hard limit
	maxRetries       = 3  // Retries for unprocessed items
)

type TelemetryStore struct {
	Client    *dynamodb.Client
	TableName string
}

// NewTelemetryStore initializes the store using the shared db.Client
func NewTelemetryStore() (*TelemetryStore, error) {
	tableName := os.Getenv("DYNAMODB_TABLE_NAME")
	if tableName == "" {
		return nil, fmt.Errorf("DYNAMODB_TABLE_NAME environment variable is not set")
	}

	// We use the global 'db.Client' we created in pkg/db/client.go
	return &TelemetryStore{
		Client:    db.Client,
		TableName: tableName,
	}, nil
}
//write to db
func (store *TelemetryStore) SaveTelemetry(ctx context.Context, data models.Telemetry) error {
	if data.ExpiresAt == 0 {
		data.ExpiresAt = time.Now().Add(7 * 24 * time.Hour).Unix()
	}

	item, err := attributevalue.MarshalMap(data)

	if err != nil {
		return fmt.Errorf("failed to marshal telemetry data: %v", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(store.TableName),
		Item:      item,
	}

	_, err = store.Client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to store data into DynamoDB: %v", err)
	}

	return nil
}

// storing multiple telemetry records in a single DynamoDB call(max 25)
func (store *TelemetryStore) SaveTelemetryBatch(ctx context.Context, dataList []models.Telemetry) error {
	if len(dataList) == 0 {
		return nil
	}

	defaultExpiry := time.Now().Add(7 * 24 * time.Hour).Unix()

	for i := 0; i < len(dataList); i += dynamoBatchLimit {

		end := i + dynamoBatchLimit
		if end > len(dataList) {
			end = len(dataList)
		}

		chunk := dataList[i:end]

		var writeRequests []types.WriteRequest

		for _, data := range chunk {
			if data.ExpiresAt == 0 {
				data.ExpiresAt = defaultExpiry
			}

			item, err := attributevalue.MarshalMap(data)
			if err != nil {
				return fmt.Errorf("failed to marshal batch item: %w", err)
			}

			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: item,
				},
			})
		}

		//use retry logic
		if err := store.writeBatchWithRetry(ctx, writeRequests); err != nil {
			return err
		}
	}

	return nil
}

func (store *TelemetryStore) writeBatchWithRetry(ctx context.Context, requests []types.WriteRequest) error {
	pending := requests

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				store.TableName: pending,
			},
		}

		output, err := store.Client.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("batch write attempt %d failed: %w", attempt+1, err)
		}

		// Check for unprocessed items
		unprocessed := output.UnprocessedItems[store.TableName]
		if len(unprocessed) == 0 {
			return nil
		}

		pending = unprocessed
	}

	return fmt.Errorf("batch write: %d items still unprocessed after %d retries", len(pending), maxRetries)
}


//get recent readings for a device.
func (store *TelemetryStore) GetTelemetryHistory(ctx context.Context, deviceID string, limit int32, since int64) ([]models.Telemetry, error) {
    keyCondition := "device_id = :id"
    exprAttrValues := map[string]types.AttributeValue{
        ":id": &types.AttributeValueMemberS{Value: deviceID},
    }

    input := &dynamodb.QueryInput{
        TableName:                 aws.String(store.TableName),
        KeyConditionExpression:    aws.String(keyCondition),
        ExpressionAttributeValues: exprAttrValues,
        ScanIndexForward:          aws.Bool(false),
    }

    if since > 0 {
        input.KeyConditionExpression = aws.String("device_id = :id AND #ts >= :since")
        input.ExpressionAttributeNames = map[string]string{"#ts": "timestamp"}
        input.ExpressionAttributeValues[":since"] = &types.AttributeValueMemberN{Value: fmt.Sprint(since)}
    }

    if limit > 0 {
        input.Limit = aws.Int32(limit)
    }

    result, err := store.Client.Query(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("failed to query telemetry history for device %s: %w", deviceID, err)
    }

    var history []models.Telemetry
    if err = attributevalue.UnmarshalListOfMaps(result.Items, &history); err != nil {
        return nil, fmt.Errorf("failed to unmarshal telemetry history for device %s: %w", deviceID, err)
    }

    return history, nil
}