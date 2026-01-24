package handlers

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/realtime"
)

type HealthHandlers struct {
	db          *database.DB
	broker      *realtime.Broker
	funcService *functions.Service
	version     string
}

func NewHealthHandlers(db *database.DB, broker *realtime.Broker, funcService *functions.Service, version string) *HealthHandlers {
	return &HealthHandlers{
		db:          db,
		broker:      broker,
		funcService: funcService,
		version:     version,
	}
}

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type ComponentHealth struct {
	Status  HealthStatus `json:"status"`
	Latency string       `json:"latency,omitempty"`
	Message string       `json:"message,omitempty"`
}

type HealthResponse struct {
	Status     HealthStatus               `json:"status"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
	Timestamp  string                     `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components"`
}

var startTime = time.Now()

const healthCheckTimeout = 5 * time.Second

func (h *HealthHandlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	components := make(map[string]ComponentHealth)
	overallStatus := HealthStatusHealthy

	dbHealth := h.checkDatabase(ctx)
	components["database"] = dbHealth
	if dbHealth.Status != HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	if h.broker != nil {
		brokerHealth := h.checkBroker()
		components["realtime"] = brokerHealth
		if brokerHealth.Status != HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	if h.funcService != nil {
		funcHealth := h.checkFunctions()
		components["functions"] = funcHealth
		if funcHealth.Status != HealthStatusHealthy && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	resp := HealthResponse{
		Status:     overallStatus,
		Version:    h.version,
		Uptime:     time.Since(startTime).Round(time.Second).String(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Components: components,
	}

	status := http.StatusOK
	if overallStatus == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	}

	JSON(w, status, resp)
}

func (h *HealthHandlers) checkDatabase(ctx context.Context) ComponentHealth {
	start := time.Now()
	err := h.db.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Latency: latency.String(),
			Message: "database ping failed",
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Latency: latency.String(),
	}
}

func (h *HealthHandlers) checkBroker() ComponentHealth {
	if h.broker == nil {
		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "disabled",
		}
	}

	stats := h.broker.Stats()
	if stats.Connections == 0 && stats.Subscriptions == 0 {
		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "no active connections",
		}
	}

	return ComponentHealth{
		Status: HealthStatusHealthy,
	}
}

func (h *HealthHandlers) checkFunctions() ComponentHealth {
	if h.funcService == nil {
		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "disabled",
		}
	}

	funcs := h.funcService.ListFunctions()
	if len(funcs) == 0 {
		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "no functions loaded",
		}
	}

	return ComponentHealth{
		Status: HealthStatusHealthy,
	}
}

func (h *HealthHandlers) Liveness(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (h *HealthHandlers) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		JSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "database unavailable",
		})
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

type RuntimeStats struct {
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
	NumCPU       int    `json:"num_cpu"`
	MemAlloc     uint64 `json:"mem_alloc_bytes"`
	MemSys       uint64 `json:"mem_sys_bytes"`
	NumGC        uint32 `json:"num_gc"`
}

func (h *HealthHandlers) Stats(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := RuntimeStats{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		MemAlloc:     m.Alloc,
		MemSys:       m.Sys,
		NumGC:        m.NumGC,
	}

	resp := map[string]any{
		"runtime": stats,
		"uptime":  time.Since(startTime).Round(time.Second).String(),
	}

	if h.db != nil {
		dbStats := h.db.Stats()
		resp["database"] = map[string]any{
			"open_connections": dbStats.OpenConnections,
			"in_use":           dbStats.InUse,
			"idle":             dbStats.Idle,
			"max_open":         dbStats.MaxOpenConnections,
		}
	}

	if h.broker != nil {
		brokerStats := h.broker.Stats()
		resp["realtime"] = map[string]any{
			"connections":   brokerStats.Connections,
			"subscriptions": brokerStats.Subscriptions,
		}
	}

	if h.funcService != nil {
		poolStats := h.funcService.Stats()
		funcStats := make(map[string]any)
		for rt, ps := range poolStats {
			funcStats[string(rt)] = map[string]any{
				"ready": ps.Ready,
				"busy":  ps.Busy,
				"total": ps.Total,
			}
		}
		resp["functions"] = funcStats
	}

	JSON(w, http.StatusOK, resp)
}
