package httpapi

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"envelope-backend/internal/domain"
	"envelope-backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo repository.Repository
}

func NewHandler(repo repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "envelope-backend", "status": "ok"})
	})
	router.GET("/api/v1/health", h.health)

	api := router.Group("/api/v1")
	{
		api.GET("/envelopes", h.listEnvelopes)
		api.POST("/envelopes", h.createEnvelope)
		api.GET("/envelopes/:id", h.getEnvelope)
		api.PUT("/envelopes/:id", h.updateEnvelope)
		api.DELETE("/envelopes/:id", h.deleteEnvelope)
		api.PUT("/allocations", h.setAllocations)

		api.POST("/transactions/income", h.createIncome)
		api.POST("/transactions/expense", h.createExpense)
		api.GET("/transactions", h.listTransactions)
		api.GET("/transactions/:id", h.getTransaction)
	}
}

func (h *Handler) health(c *gin.Context) {
	if err := h.repo.Ping(c.Request.Context()); err != nil {
		writeError(c, http.StatusServiceUnavailable, "database_unavailable", "База данных недоступна")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) listEnvelopes(c *gin.Context) {
	items, err := h.repo.ListEnvelopes(c.Request.Context())
	if err != nil {
		writeInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) getEnvelope(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.repo.GetEnvelope(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

type envelopeRequest struct {
	Name     string `json:"name" binding:"required,max=80"`
	Category string `json:"category" binding:"required,max=40"`
}

func (h *Handler) createEnvelope(c *gin.Context) {
	var req envelopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Нужны name (до 80 символов) и category (до 40 символов)")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	if req.Name == "" || req.Category == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "Название и категория не должны быть пустыми")
		return
	}
	item, err := h.repo.CreateEnvelope(c.Request.Context(), req.Name, req.Category)
	if err != nil {
		writeInternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *Handler) updateEnvelope(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req envelopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Нужны name (до 80 символов) и category (до 40 символов)")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	if req.Name == "" || req.Category == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "Название и категория не должны быть пустыми")
		return
	}
	item, err := h.repo.UpdateEnvelope(c.Request.Context(), id, req.Name, req.Category)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *Handler) deleteEnvelope(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.repo.DeleteEnvelope(c.Request.Context(), id); err != nil {
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type allocationsRequest struct {
	Allocations []domain.AllocationUpdate `json:"allocations" binding:"required,min=1,dive"`
}

func (h *Handler) setAllocations(c *gin.Context) {
	var req allocationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Передайте массив allocations с envelope_id и percent для каждого конверта")
		return
	}
	items, err := h.repo.SetAllocations(c.Request.Context(), req.Allocations)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

type incomeRequest struct {
	AmountKopecks int64  `json:"amount_kopecks" binding:"required,gt=0"`
	Description   string `json:"description" binding:"max=200"`
}

func (h *Handler) createIncome(c *gin.Context) {
	var req incomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "amount_kopecks должен быть положительным числом; description — не более 200 символов")
		return
	}
	item, err := h.repo.CreateIncome(c.Request.Context(), req.AmountKopecks, req.Description)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

type expenseRequest struct {
	EnvelopeID    int64  `json:"envelope_id" binding:"required,gt=0"`
	AmountKopecks int64  `json:"amount_kopecks" binding:"required,gt=0"`
	Description   string `json:"description" binding:"max=200"`
}

func (h *Handler) createExpense(c *gin.Context) {
	var req expenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Нужны положительные envelope_id и amount_kopecks; description — не более 200 символов")
		return
	}
	item, err := h.repo.CreateExpense(c.Request.Context(), req.EnvelopeID, req.AmountKopecks, req.Description)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *Handler) listTransactions(c *gin.Context) {
	limit := 20
	offset := 0
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(c, http.StatusBadRequest, "invalid_query", "limit должен быть от 1 до 100")
			return
		}
		limit = parsed
	}
	if raw := c.Query("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(c, http.StatusBadRequest, "invalid_query", "offset должен быть неотрицательным числом")
			return
		}
		offset = parsed
	}
	items, err := h.repo.ListTransactions(c.Request.Context(), limit, offset)
	if err != nil {
		writeInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"limit": limit, "offset": offset}})
}

func (h *Handler) getTransaction(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.repo.GetTransaction(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_id", "ID должен быть положительным целым числом")
		return 0, false
	}
	return id, true
}

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "Объект не найден")
	case errors.Is(err, domain.ErrInvalidAmount):
		writeError(c, http.StatusBadRequest, "invalid_amount", "Сумма должна быть положительной")
	case errors.Is(err, domain.ErrInvalidAllocation):
		writeError(c, http.StatusBadRequest, "invalid_allocation", "Проценты должны в сумме давать 100%, каждый конверт нужно передать ровно один раз")
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(c, http.StatusConflict, "insufficient_funds", "В конверте недостаточно денег")
	case errors.Is(err, domain.ErrEnvelopeHasBalance):
		writeError(c, http.StatusConflict, "envelope_has_balance", "Сначала потратьте или перераспределите остаток конверта")
	case errors.Is(err, domain.ErrEnvelopeHasHistory):
		writeError(c, http.StatusConflict, "envelope_has_history", "Нельзя удалить конверт, по которому уже есть операции")
	case errors.Is(err, domain.ErrAllocationNotZero):
		writeError(c, http.StatusConflict, "allocation_not_zero", "Перед удалением установите процент конверта в 0")
	default:
		writeInternalError(c, err)
	}
}

func writeInternalError(c *gin.Context, err error) {
	// Keep internal details in server logs, but do not expose them to API clients.
	log.Printf("internal API error: %v", err)
	writeError(c, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
