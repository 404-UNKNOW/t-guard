package mocks

import (
	"context"
	"errors"
	"t-guard/pkg/store"
)

type MockStore struct {
	Records  []store.Record
	FailNext bool
}

func (m *MockStore) Write(ctx context.Context, r store.Record) error {
	if m.FailNext {
		m.FailNext = false // 重置
		return errors.New("mock storage failure")
	}
	m.Records = append(m.Records, r)
	return nil
}

func (m *MockStore) GetDailyStats(ctx context.Context, project, date string) (store.DailyStats, error) {
	return store.DailyStats{Project: project, Date: date}, nil
}

func (m *MockStore) GetRecentRequests(ctx context.Context, project string, limit int) ([]store.Record, error) {
	return m.Records, nil
}

func (m *MockStore) QueryProjects(ctx context.Context) ([]string, error) {
	return []string{"test-project"}, nil
}

func (m *MockStore) DeductBudget(ctx context.Context, r store.Record, limit int64) error {
	return nil
}

func (m *MockStore) Archive(ctx context.Context, beforeDate string) (int, error) {
	return 0, nil
}

func (m *MockStore) Close() error {
	return nil
}
