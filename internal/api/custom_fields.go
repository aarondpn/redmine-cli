package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// CustomFieldService handles custom field definition API calls. Redmine only
// exposes the list endpoint (admin only), so Get resolves from the list.
type CustomFieldService struct {
	client *Client
}

// List retrieves all custom field definitions.
func (s *CustomFieldService) List(ctx context.Context) ([]models.CustomField, error) {
	var resp struct {
		CustomFields []models.CustomField `json:"custom_fields"`
	}
	if err := s.client.Get(ctx, "/custom_fields.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.CustomFields, nil
}

// Get retrieves a single custom field definition by ID by resolving it from
// the list endpoint, which is the only custom-field endpoint Redmine exposes.
func (s *CustomFieldService) Get(ctx context.Context, id int) (*models.CustomField, error) {
	fields, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range fields {
		if fields[i].ID == id {
			return &fields[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Errors:     []string{fmt.Sprintf("custom field %d not found", id)},
	}
}
