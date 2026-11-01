package notifications

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"envelope-backend/internal/domain"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes добавляет маршруты в группу /api/v1.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/notifications", h.list)
	api.POST("/notifications/:id/read", h.markRead)
	api.POST("/notifications/:id/action", h.act)
	api.GET("/survey/questions", h.questions)
	api.POST("/survey", h.submitSurvey)
}

// list возвращает уведомления. С параметром refresh=1 перед ответом выполняется
// внеплановая проверка правил — фронтенд использует его сразу после своих операций.
func (h *Handler) list(c *gin.Context) {
	if c.Query("refresh") == "1" {
		if err := h.svc.Scan(c.Request.Context()); err != nil {
			log.Printf("notifications: on-demand scan failed: %v", err)
		}
	}
	unreadOnly := c.Query("unread") == "1" || c.Query("unread") == "true"
	items, unread := h.svc.List(unreadOnly)
	c.JSON(http.StatusOK, gin.H{"data": items, "unread_count": unread})
}

func (h *Handler) markRead(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.svc.MarkRead(id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

type actionRequest struct {
	Action string `json:"action" binding:"required"`
}

func (h *Handler) act(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req actionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Нужно поле action")
		return
	}
	item, err := h.svc.Act(c.Request.Context(), id, req.Action)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *Handler) questions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": Questions()})
}

type surveyRequest struct {
	Answers Answers `json:"answers"`
}

func (h *Handler) submitSurvey(c *gin.Context) {
	var req surveyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Передайте объект answers: номер вопроса → список номеров вариантов")
		return
	}
	item, err := h.svc.SubmitSurvey(req.Answers)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_id", "ID должен быть положительным целым числом")
		return 0, false
	}
	return id, true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotificationNotFound), errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "Объект не найден")
	case errors.Is(err, ErrUnknownAction):
		writeError(c, http.StatusBadRequest, "unknown_action", "Это действие недоступно для данного уведомления")
	case errors.Is(err, ErrInvalidAnswers):
		writeError(c, http.StatusBadRequest, "invalid_answers", err.Error())
	case errors.Is(err, ErrAlreadyResolved):
		writeError(c, http.StatusConflict, "already_resolved", "Уведомление уже обработано")
	case errors.Is(err, ErrNothingToMove):
		writeError(c, http.StatusConflict, "nothing_to_move", "Нет конвертов, у которых можно забрать часть доли")
	case errors.Is(err, domain.ErrInvalidAllocation):
		writeError(c, http.StatusConflict, "allocation_conflict", "Набор конвертов изменился. Обновите страницу и повторите действие")
	default:
		log.Printf("notifications: internal error: %v", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
