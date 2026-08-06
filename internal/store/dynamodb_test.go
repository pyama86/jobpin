package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const testTable = "jobpin_test"

func newTestStore(t *testing.T) *DynamoDB {
	t.Helper()

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Setenv("AWS_REGION", "ap-northeast-1")

	s, err := NewDynamoDB(ctx, testTable, endpoint)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if _, err := s.client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)}); err != nil {
		t.Skipf("DynamoDB Local (%s) に接続できないためスキップ: %v", endpoint, err)
	}
	return s
}

func TestDynamoDBLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.EnsureTable(ctx); err != nil {
		t.Fatalf("EnsureTable failed: %v", err)
	}

	now := time.Now()
	// 他のテスト実行の残骸と衝突しないようユニークなIDを使う
	threadTS := fmt.Sprintf("%d.000001", now.UnixNano())
	job := &Job{
		ID:        NewJobID("pyama86", "jobpin", 123, "C012345", threadTS),
		Owner:     "pyama86",
		Repo:      "jobpin",
		RunID:     123,
		Channel:   "C012345",
		ThreadTS:  threadTS,
		Requester: "U012345",
		Status:    StatusWatching,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour).Unix(),
	}

	if err := s.Put(ctx, job); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	jobs, err := s.ListWatching(ctx)
	if err != nil {
		t.Fatalf("ListWatching failed: %v", err)
	}
	if !containsJobID(jobs, job.ID) {
		t.Fatalf("ListWatching does not contain %s", job.ID)
	}

	if err := s.UpdateStatus(ctx, job.ID, StatusNotified); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	jobs, err = s.ListWatching(ctx)
	if err != nil {
		t.Fatalf("ListWatching after update failed: %v", err)
	}
	if containsJobID(jobs, job.ID) {
		t.Fatalf("ListWatching should not contain notified job %s", job.ID)
	}
}

func containsJobID(jobs []*Job, id string) bool {
	for _, j := range jobs {
		if j.ID == id {
			return true
		}
	}
	return false
}
