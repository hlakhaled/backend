package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/Fleexa-Graduation-Project/Backend/models"
	"github.com/Fleexa-Graduation-Project/Backend/pkg/db"
)

type CommandStore struct {
	Client    *dynamodb.Client
	TableName string
}

func NewCommandStore() (*CommandStore, error) {
	tableName := os.Getenv("DYNAMODB_COMMANDS_TABLE")
	if tableName == "" {
		return nil, fmt.Errorf("DYNAMODB_COMMANDS_TABLE environment variable is not set")
	}

	if db.Client == nil {
		return nil, fmt.Errorf("dynamodb client is not initialized")
	}

	return &CommandStore{
		Client:    db.Client,
		TableName: tableName,
	}, nil
}

func (store *CommandStore) SaveCommand(ctx context.Context, cmd models.Command) error {
	if cmd.ExpiresAt == 0 {
		cmd.ExpiresAt = time.Now().Add(30 * 24 * time.Hour).Unix()
	}

	if cmd.Timestamp == 0 {
		cmd.Timestamp = time.Now().Unix()
	}

	item, err := attributevalue.MarshalMap(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(store.TableName),
		Item:      item,
	}

	_, err = store.Client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to store command in dynamodb: %w", err)
	}

	return nil
}
