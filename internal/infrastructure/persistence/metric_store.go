package persistence

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// MetricStore provides database persistence for metrics.
type MetricStore struct {
	db *gorm.DB
}

// NewMetricStore creates a new MetricStore.
func NewMetricStore(db *gorm.DB) *MetricStore {
	return &MetricStore{db: db}
}

// SaveMetric saves a metric entry to the database.
func (s *MetricStore) SaveMetric(ctx context.Context, metricName, metricType string, value float64, labels map[string]string) error {
	var labelsData []byte
	var err error
	if labels != nil {
		labelsData, err = json.Marshal(labels)
		if err != nil {
			return err
		}
	}

	metric := &MetricModel{
		Timestamp:  time.Now(),
		MetricName: metricName,
		MetricType: metricType,
		Value:      value,
		Labels:     labelsData,
		CreatedAt:  time.Now(),
	}

	return s.db.WithContext(ctx).Create(metric).Error
}

// GetMetricsByName retrieves metrics by name with optional time range.
func (s *MetricStore) GetMetricsByName(ctx context.Context, metricName string, since time.Time) ([]MetricModel, error) {
	var metrics []MetricModel
	err := s.db.WithContext(ctx).
		Where("metric_name = ? AND created_at >= ?", metricName, since).
		Order("created_at DESC").
		Find(&metrics).Error
	return metrics, err
}

// AggregateMetric aggregates metric values (e.g., sum, avg, max).
func (s *MetricStore) AggregateMetric(ctx context.Context, metricName, aggregation string, since time.Time) (float64, error) {
	var result float64
	query := s.db.WithContext(ctx).
		Table("metrics").
		Where("metric_name = ? AND created_at >= ?", metricName, since)

	switch aggregation {
	case "sum":
		query = query.Select("COALESCE(SUM(value), 0)")
	case "avg":
		query = query.Select("COALESCE(AVG(value), 0)")
	case "max":
		query = query.Select("COALESCE(MAX(value), 0)")
	case "min":
		query = query.Select("COALESCE(MIN(value), 0)")
	case "count":
		query = query.Select("COUNT(*)")
	default:
		query = query.Select("COALESCE(SUM(value), 0)")
	}

	err := query.Row().Scan(&result)
	return result, err
}

// DeleteOldMetrics deletes metrics older than the specified duration.
func (s *MetricStore) DeleteOldMetrics(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return s.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&MetricModel{}).Error
}
