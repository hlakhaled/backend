package devices

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/Fleexa-Graduation-Project/Backend/models"
	"github.com/Fleexa-Graduation-Project/Backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

const (
	OfflineLimit = 2 * time.Minute
)

type StateStore struct {
	Client    *dynamodb.Client
	TableName string
}

func NewStateStore() (*StateStore, error) {
	tableName := os.Getenv("DYNAMODB_DEVICE_STATE_TABLE")
	if tableName == "" {
		return nil, fmt.Errorf("DYNAMODB_DEVICE_STATE_TABLE is not set")
	}

	if db.Client == nil {
		return nil, fmt.Errorf("dynamodb client not initialized")
	}

	return &StateStore{
		Client:    db.Client,
		TableName: tableName,
	}, nil
}
//updates live dashboard
func (s *StateStore) UpdateFromTelemetry(ctx context.Context,tel models.Telemetry,) error {

	now := time.Now().Unix()
	
	opState, health := ExtractState(tel.Type, tel.Payload)
	status := "ONLINE"
	payload, err := attributevalue.Marshal(tel.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(s.TableName),
		Key: map[string]types.AttributeValue{
			"device_id": &types.AttributeValueMemberS{
				Value: tel.DeviceID,
			},
		},
		ConditionExpression: aws.String(
            "attribute_not_exists(last_seen_at) OR last_seen_at <= :last_seen",
        ),
		UpdateExpression: aws.String(`
			SET 
				#type = :type,
				#status = :status,
				operational_state = :op_state,
				health = :health,
				payload = :payload,
				last_seen_at = :last_seen,
				updated_at = :updated_at
		`),
		ExpressionAttributeNames: map[string]string{
			"#type":   "type",
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":type":       &types.AttributeValueMemberS{Value: tel.Type},
			":status":     &types.AttributeValueMemberS{Value: status},
			":op_state":   &types.AttributeValueMemberS{Value: opState},
			":health":     &types.AttributeValueMemberS{Value: health},
			":payload":    payload, // This stores the raw map (temp, gas_level, etc.)
			":last_seen":  &types.AttributeValueMemberN{Value: fmt.Sprint(tel.Timestamp)},
			":updated_at": &types.AttributeValueMemberN{Value: fmt.Sprint(now)},
		},
	}

	_, err = s.Client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update device state: %w", err)
	}

	return nil
}



//extracting op state and health from the device type and its payload
func ExtractState(deviceType string,payload map[string]interface{},) (string, string)  {

	deviceRules, ok := Rules[deviceType]
opState := "UNKNOWN"
	health := "DEGRADED"

	if ok {
		opState = deviceRules.ExtractOperational(payload)
		health = deviceRules.EvaluateHealth(opState)
	}

	return opState, health
}

func (s *StateStore) UpdateHeartbeat(ctx context.Context,deviceID string,) error {
    now := time.Now().Unix()

    input := &dynamodb.UpdateItemInput{
        TableName: aws.String(s.TableName),
        Key: map[string]types.AttributeValue{
            "device_id": &types.AttributeValueMemberS{Value: deviceID},
        },
		ConditionExpression: aws.String(
            "attribute_not_exists(last_seen_at) OR last_seen_at <= :last_seen",
        ),
        UpdateExpression: aws.String(
            "SET #status = :status, last_seen_at = :last_seen",
        ),
        ExpressionAttributeNames: map[string]string{
            "#status": "status",
        },
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":status":    &types.AttributeValueMemberS{Value: "ONLINE"},
            ":last_seen": &types.AttributeValueMemberN{Value: fmt.Sprint(now)},
        },
    }

    _, err := s.Client.UpdateItem(ctx, input)
    return err
}

func ConnectionStatus(lastSeenAt int64) string {
	if time.Since(time.Unix(lastSeenAt, 0)) > OfflineLimit {
		return "OFFLINE"
	}
	return "ONLINE"
}

// retrieve all devices states for the dashboard
func (store *StateStore) GetAllStates(ctx context.Context) ([]models.DeviceState, error) {
	var states []models.DeviceState
	
	var lastEvaluatedKey map[string]types.AttributeValue

	// Keep scanning until we get all devices
	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(store.TableName),
			ExclusiveStartKey: lastEvaluatedKey, // Start where the last page left off
		}

		result, err := store.Client.Scan(ctx, input)  // works as select * w/o WHERE clause
		if err != nil {
			return nil, fmt.Errorf("failed to scan device states: %w", err)
		}

		for _, item := range result.Items {
			var state models.DeviceState
			err = attributevalue.UnmarshalMap(item, &state)
			if err != nil {
				fmt.Printf("Warning: failed to unmarshal a device state: %v\n", err)
				continue
			}
			states = append(states, state)
		}
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break 
	}
}

	return states, nil
}



// retrieve the device state by id
func (s *StateStore) GetStateByID(ctx context.Context, deviceID string) (*models.DeviceState, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(s.TableName),
		Key: map[string]types.AttributeValue{
			"device_id": &types.AttributeValueMemberS{Value: deviceID},
		},
	}

	result, err := s.Client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	// If the device doesn't exist, return an empty item map
	if result.Item == nil {
		return nil, nil 
	}


	var state models.DeviceState
	err = attributevalue.UnmarshalMap(result.Item, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal device state: %w", err)
	}

	return &state, nil
}