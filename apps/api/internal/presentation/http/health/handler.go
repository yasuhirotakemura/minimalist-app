// Package health は監視用endpointを提供する (設計書 23.2)。
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// readinessCheckTimeout はDB疎通確認の上限時間。
const readinessCheckTimeout = 2 * time.Second

// Pinger はDBへの疎通確認を行う。
type Pinger interface {
	Ping(ctx context.Context) error
}

// StatusResponse は監視endpointのresponse。
type StatusResponse struct {
	Status string `json:"status"`
}

// Handler は監視endpointのHTTP handlerである。
type Handler struct {
	pinger Pinger
}

// NewHandler はHandlerを生成する。
func NewHandler(pinger Pinger) *Handler {
	return &Handler{pinger: pinger}
}

// Live はprocessが起動していることを返す。
// GET /health/live
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(r.Context(), w, http.StatusOK, StatusResponse{Status: "ok"})
}

// Ready はrequestを処理できる状態かを返す。
// GET /health/ready
//
// DB接続不能時はreadinessを失敗させる (設計書 23.2)。
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readinessCheckTimeout)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		logging.FromContext(r.Context()).Error("readiness check failed", "error", err.Error())
		shared.WriteJSON(r.Context(), w, http.StatusServiceUnavailable,
			StatusResponse{Status: "unavailable"})
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, StatusResponse{Status: "ok"})
}
