package store

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const tableActiveWaitTimeout = 60 * time.Second

type DynamoDB struct {
	client *dynamodb.Client
	table  string
}

var _ Store = (*DynamoDB)(nil)

func NewDynamoDB(ctx context.Context, table, endpoint string) (*DynamoDB, error) {
	var loadOpts []func(*config.LoadOptions) error
	if endpoint != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	var clientOpts []func(*dynamodb.Options)
	if endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	return &DynamoDB{
		client: dynamodb.NewFromConfig(awsCfg, clientOpts...),
		table:  table,
	}, nil
}

func (d *DynamoDB) Put(ctx context.Context, job *Job) error {
	item, err := attributevalue.MarshalMap(job)
	if err != nil {
		return err
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.table),
		Item:      item,
	})
	return err
}

func (d *DynamoDB) ListWatching(ctx context.Context) ([]*Job, error) {
	var jobs []*Job
	var startKey map[string]types.AttributeValue

	for {
		out, err := d.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:        aws.String(d.table),
			FilterExpression: aws.String("#status = :status"),
			ExpressionAttributeNames: map[string]string{
				"#status": "status",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":status": &types.AttributeValueMemberS{Value: StatusWatching},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}

		var page []*Job
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &page); err != nil {
			return nil, err
		}
		jobs = append(jobs, page...)

		if out.LastEvaluatedKey == nil {
			return jobs, nil
		}
		startKey = out.LastEvaluatedKey
	}
}

func (d *DynamoDB) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(d.table),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: status},
		},
	})
	return err
}

func (d *DynamoDB) EnsureTable(ctx context.Context) error {
	_, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.table),
	})
	if err == nil {
		return nil
	}
	var notFound *types.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return err
	}

	_, err = d.client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(d.table),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		return err
	}

	waiter := dynamodb.NewTableExistsWaiter(d.client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.table),
	}, tableActiveWaitTimeout); err != nil {
		return err
	}

	_, err = d.client.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(d.table),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			AttributeName: aws.String("expires_at"),
			Enabled:       aws.Bool(true),
		},
	})
	return err
}
