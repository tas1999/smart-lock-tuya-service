package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tas1999/smart-lock-tuya-service/internal/config"
	"github.com/tas1999/smart-lock-tuya-service/internal/webhook"
	"github.com/tas1999/tuya-connector-go/connector/constant"
	"github.com/tas1999/tuya-connector-go/connector/env/extension"
	"github.com/tas1999/tuya-connector-go/connector/logger"
	"github.com/tas1999/tuya-connector-go/connector/message/event"
)

type Listener struct {
	Cfg     config.Config
	Webhook *webhook.Client
}

func (l *Listener) Start() {
	r := extension.GetMessage(constant.TUYA_MESSAGE)
	r.InitMessageClient()

	extension.GetMessage(constant.TUYA_MESSAGE).SubEventMessage(func(m *event.StatusReportMessage) {
		l.handleStatusReport(m)
	})
	extension.GetMessage(constant.TUYA_MESSAGE).SubEventMessage(func(m *event.DevicePropertyMessage) {
		l.handleDeviceProperty(m)
	})
	logger.Log.Info("pulsar listeners registered (StatusReport, DeviceProperty)")
}

func (l *Listener) handleStatusReport(m *event.StatusReportMessage) {
	if m == nil {
		return
	}
	deviceID := m.DevID
	if deviceID == "" {
		deviceID = m.BizData.DevID
	}
	if !l.Cfg.MatchesDevice(deviceID) {
		return
	}
	for _, item := range m.Status {
		if !l.Cfg.MatchesDP(item.Code) {
			continue
		}
		l.emit(deviceID, item.Code, item.Value, item.T)
	}
}

func (l *Listener) handleDeviceProperty(m *event.DevicePropertyMessage) {
	if m == nil {
		return
	}
	deviceID := m.DevID
	if deviceID == "" {
		deviceID = m.BizData.DevID
	}
	if !l.Cfg.MatchesDevice(deviceID) {
		return
	}
	for _, p := range m.BizData.Properties {
		if !l.Cfg.MatchesDP(p.Code) {
			continue
		}
		l.emit(deviceID, p.Code, p.Value, p.Time)
	}
}

func (l *Listener) emit(deviceID, code string, value any, ts int64) {
	occurred := time.Now().UTC()
	if ts > 0 {
		// Tuya often sends milliseconds
		if ts > 1_000_000_000_000 {
			occurred = time.UnixMilli(ts).UTC()
		} else {
			occurred = time.Unix(ts, 0).UTC()
		}
	}
	ev := webhook.Event{
		ID:         newEventID(),
		Type:       "lock.unlock",
		OccurredAt: occurred,
		DeviceID:   deviceID,
		Method:     code,
		Properties: map[string]any{
			"code":  code,
			"value": value,
		},
	}
	logger.Log.Infof("matched unlock event device=%s dp=%s value=%v", deviceID, code, value)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := l.Webhook.Post(ctx, ev); err != nil {
		logger.Log.Errorf("webhook post failed: %v", err)
	}
}

func DeviceFilterSummary(cfg config.Config) string {
	return fmt.Sprintf("devices=%v dps=%v webhook=%s", cfg.EventMatchDevices, cfg.EventMatchDPs, cfg.WebhookURL)
}

func newEventID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
