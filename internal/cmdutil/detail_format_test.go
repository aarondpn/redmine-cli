package cmdutil

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestFormatCustomFields(t *testing.T) {
	got := FormatCustomFields([]models.CustomFieldValue{
		{ID: 1, Name: "Env", Value: "prod"},
		{ID: 2, Name: "Empty", Value: ""},
		{ID: 3, Name: "Nil", Value: nil},
		{ID: 4, Name: "Multi", Value: []any{"a", "b"}, Multiple: true},
		{ID: 5, Name: "Num", Value: 3.0},
	})
	want := "Env: prod; Multi: a, b; Num: 3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if FormatCustomFields(nil) != "" {
		t.Error("nil should render empty")
	}
}
