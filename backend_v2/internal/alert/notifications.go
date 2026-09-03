package alert

import (
	"context"
	"encoding/json"
	"strings"

	coreservice "aladin/backend_v2/internal/service"
)

// Notification is one durable per-user inbox item. It is a reusable primitive: alerts are the
// first producer (Kind "price_alert"); insights/fills/copilot can reuse it unchanged.
type Notification struct {
	ID        string          `json:"id"`
	UserID    string          `json:"userId"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data,omitempty"`
	ReadAt    string          `json:"readAt,omitempty"`
	CreatedAt string          `json:"createdAt"`
}

// NotificationRepository persists notifications. Create is TRANSACTIONAL + self-transporting:
// it inserts the row AND appends the outbox app_event in one tx, so every producer gets
// durability (the table) + live delivery (via the OutboxDrainer) with no realtime coupling.
type NotificationRepository interface {
	Create(ctx context.Context, n Notification) (Notification, error)
	List(ctx context.Context, userID string, limit int) ([]Notification, error)
	ListUnread(ctx context.Context, userID string) ([]Notification, error)
	MarkRead(ctx context.Context, userID, id string) error
}

// NotificationService is the reusable notification primitive.
type NotificationService interface {
	Create(ctx context.Context, n Notification) (Notification, error)
	List(ctx context.Context, userID string, limit int) ([]Notification, error)
	ListUnread(ctx context.Context, userID string) ([]Notification, error)
	MarkRead(ctx context.Context, userID, id string) error
}

const notificationDefaultLimit = 50

type defaultNotificationService struct {
	repo NotificationRepository
}

func NewNotificationService(repo NotificationRepository) NotificationService {
	return &defaultNotificationService{repo: repo}
}

func (s *defaultNotificationService) Create(ctx context.Context, n Notification) (Notification, error) {
	if strings.TrimSpace(n.UserID) == "" {
		return Notification{}, coreservice.BadRequest("notification requires a user id")
	}
	if strings.TrimSpace(n.Kind) == "" || strings.TrimSpace(n.Title) == "" {
		return Notification{}, coreservice.BadRequest("notification requires a kind and title")
	}
	return s.repo.Create(ctx, n)
}

func (s *defaultNotificationService) List(ctx context.Context, userID string, limit int) ([]Notification, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, coreservice.ErrUnauthenticated
	}
	if limit <= 0 || limit > 200 {
		limit = notificationDefaultLimit
	}
	items, err := s.repo.List(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Notification{}
	}
	return items, nil
}

func (s *defaultNotificationService) ListUnread(ctx context.Context, userID string) ([]Notification, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, coreservice.ErrUnauthenticated
	}
	items, err := s.repo.ListUnread(ctx, userID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Notification{}
	}
	return items, nil
}

func (s *defaultNotificationService) MarkRead(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(userID) == "" {
		return coreservice.ErrUnauthenticated
	}
	if strings.TrimSpace(id) == "" {
		return coreservice.BadRequest("notification id is required")
	}
	return s.repo.MarkRead(ctx, userID, id)
}

// NotificationCreatedPayload is the realtime event shape (resource kind "notification",
// operation "created") the drainer delivers to the user's workspace stream.
type NotificationCreatedPayload struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data,omitempty"`
	CreatedAt string          `json:"createdAt"`
}
