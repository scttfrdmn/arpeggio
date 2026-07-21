// Package store persists portal state in a single DynamoDB table.
//
// The table is on-demand. Provisioned capacity is a clock and is banned
// (CLAUDE.md golden rule 2). Sessions carry a TTL attribute so DynamoDB reaps
// them for free — no sweeper Lambda, no schedule.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/scttfrdmn/arpeggio/internal/auth"
)

// Table is the single-table store. Keys are PK/SK strings; see docs/DATA_MODEL.md.
type Table struct {
	client *dynamodb.Client
	name   string
}

// New returns a Table backed by the given client.
func New(client *dynamodb.Client, name string) *Table {
	return &Table{client: client, name: name}
}

type sessionItem struct {
	PK string `dynamodbav:"pk"`
	SK string `dynamodbav:"sk"`
	auth.Session
}

func sessionKey(id string) (string, string) { return "SESSION#" + id, "META" }

// Put stores a session.
func (t *Table) Put(ctx context.Context, s *auth.Session) error {
	pk, sk := sessionKey(s.ID)
	item, err := attributevalue.MarshalMap(sessionItem{PK: pk, SK: sk, Session: *s})
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	_, err = t.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(t.name),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put session: %w", err)
	}
	return nil
}

// Get loads a session by ID.
func (t *Table) Get(ctx context.Context, id string) (*auth.Session, error) {
	pk, sk := sessionKey(id)
	out, err := t.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(t.name),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if out.Item == nil {
		return nil, auth.ErrNoSession
	}
	var item sessionItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &item.Session, nil
}

// Delete removes a session.
func (t *Table) Delete(ctx context.Context, id string) error {
	pk, sk := sessionKey(id)
	_, err := t.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(t.name),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil && !errors.Is(err, auth.ErrNotFound) {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
