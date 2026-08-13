package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	checks := map[string]string{}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		checks["database"] = fmt.Sprintf("error: %v", err)
		status = http.StatusServiceUnavailable
	} else if err := sqlDB.PingContext(ctx); err != nil {
		checks["database"] = fmt.Sprintf("error: %v", err)
		status = http.StatusServiceUnavailable
	} else {
		checks["database"] = "ok"
	}

	peerCtx, peerCancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer peerCancel()

	peerAddr := r.URL.Query().Get("peerAddr")
	if peerAddr == "" {
		peerAddr = "localhost:5001"
	}

	var d net.Dialer
	conn, err := d.DialContext(peerCtx, "tcp", peerAddr)
	if err != nil {
		checks["peerjs"] = fmt.Sprintf("error: %v", err)
		status = http.StatusServiceUnavailable
	} else {
		conn.Close()
		checks["peerjs"] = "ok"
	}

	writeJSON(w, status, map[string]interface{}{
		"status":  http.StatusText(status),
		"checks":  checks,
		"peerAddr": peerAddr,
	})
}
