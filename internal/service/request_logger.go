package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"channel-adapter-gateway/internal/config"
	"channel-adapter-gateway/internal/model"

	"gorm.io/gorm"
)

type RequestLogger struct {
	db      *gorm.DB
	cfg     config.LoggingConfig
	queue   chan model.RequestLog
	stopCh  chan struct{}
	wg      sync.WaitGroup
	stopped atomic.Bool
}

func NewRequestLogger(db *gorm.DB, cfg config.LoggingConfig) *RequestLogger {
	logger := &RequestLogger{
		db:     db,
		cfg:    cfg,
		queue:  make(chan model.RequestLog, cfg.QueueSize),
		stopCh: make(chan struct{}),
	}
	if cfg.IsAsyncRequestLog() {
		for i := 0; i < cfg.WorkerCount; i++ {
			logger.wg.Add(1)
			go logger.worker()
		}
	}
	return logger
}

func (l *RequestLogger) Enqueue(row model.RequestLog) {
	if l == nil {
		return
	}
	if !l.cfg.IsAsyncRequestLog() {
		l.writeWithRetry([]model.RequestLog{row})
		return
	}
	if l.stopped.Load() {
		l.writeWithRetry([]model.RequestLog{row})
		return
	}

	timeout := time.Duration(l.cfg.EnqueueTimeoutMs) * time.Millisecond
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case l.queue <- row:
	case <-timer.C:
		if l.cfg.LogDroppedWhenFull {
			log.Printf("request log queue full, dropped request_id=%s", row.RequestID)
		}
	case <-l.stopCh:
		l.writeWithRetry([]model.RequestLog{row})
	}
}

func (l *RequestLogger) Shutdown(ctx context.Context) {
	if l == nil || !l.cfg.IsAsyncRequestLog() {
		return
	}
	if !l.stopped.CompareAndSwap(false, true) {
		return
	}
	close(l.stopCh)
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("request logger shutdown timeout: %v", ctx.Err())
	}
}

func (l *RequestLogger) worker() {
	defer l.wg.Done()

	batch := make([]model.RequestLog, 0, l.cfg.BatchSize)
	ticker := time.NewTicker(time.Duration(l.cfg.FlushIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		rows := make([]model.RequestLog, len(batch))
		copy(rows, batch)
		batch = batch[:0]
		l.writeWithRetry(rows)
	}

	for {
		select {
		case row := <-l.queue:
			batch = append(batch, row)
			if len(batch) >= l.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-l.stopCh:
			flush()
			for {
				select {
				case row := <-l.queue:
					batch = append(batch, row)
					if len(batch) >= l.cfg.BatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (l *RequestLogger) writeWithRetry(rows []model.RequestLog) {
	if len(rows) == 0 {
		return
	}
	normalizeRequestLogJSONB(rows)
	var err error
	for attempt := 0; attempt <= l.cfg.MaxRetries; attempt++ {
		err = l.db.CreateInBatches(rows, len(rows)).Error
		if err == nil {
			return
		}
		if attempt < l.cfg.MaxRetries {
			time.Sleep(time.Duration(l.cfg.RetryIntervalMs) * time.Millisecond)
		}
	}
	log.Printf("write request logs failed after retries: rows=%d err=%v", len(rows), err)
}

func normalizeRequestLogJSONB(rows []model.RequestLog) {
	for i := range rows {
		rows[i].RequestSnapshot = normalizeJSONBString(rows[i].RequestSnapshot)
		rows[i].OfficialRequest = normalizeJSONBString(rows[i].OfficialRequest)
		rows[i].UpstreamRequest = normalizeJSONBString(rows[i].UpstreamRequest)
		rows[i].UpstreamResponse = normalizeJSONBString(rows[i].UpstreamResponse)
		rows[i].OfficialResponse = normalizeJSONBString(rows[i].OfficialResponse)
		rows[i].ResponseUsage = normalizeJSONBString(rows[i].ResponseUsage)
	}
}

func normalizeJSONBString(value string) string {
	if value == "" {
		return "null"
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		fallback, _ := json.Marshal(map[string]any{"raw": value})
		return string(fallback)
	}
	return value
}
