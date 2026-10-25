package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// TestUserOps_RedactedVariantsDropAPIKey pins the split between the CLI-facing
// user operations, which keep api_key for the account owner, and the
// MCP-facing *Redacted variants, which must never return it.
func TestUserOps_RedactedVariantsDropAPIKey(t *testing.T) {
	const key = "deadbeef123"
	user := `{"id":1,"login":"alice","api_key":"` + key + `"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users.json":
			_, _ = w.Write([]byte(`{"users":[` + user + `,` + user + `],"total_count":2}`))
		case r.Method == http.MethodPost && r.URL.Path == "/users.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"user":` + user + `}`))
		default:
			_, _ = w.Write([]byte(`{"user":` + user + `}`))
		}
	}))
	defer srv.Close()
	client := newRelationsTestClient(t, srv)
	ctx := context.Background()

	single := []struct {
		name     string
		plain    func() (*models.User, error)
		redacted func() (*models.User, error)
	}{
		{
			name:     "get_my_account",
			plain:    func() (*models.User, error) { return GetMyAccount(ctx, client, GetMyAccountInput{}) },
			redacted: func() (*models.User, error) { return GetMyAccountRedacted(ctx, client, GetMyAccountInput{}) },
		},
		{
			name:     "get_user",
			plain:    func() (*models.User, error) { return GetUser(ctx, client, GetUserInput{ID: 1}) },
			redacted: func() (*models.User, error) { return GetUserRedacted(ctx, client, GetUserInput{ID: 1}) },
		},
		{
			name:     "me",
			plain:    func() (*models.User, error) { return GetCurrentUser(ctx, client, GetCurrentUserInput{}) },
			redacted: func() (*models.User, error) { return GetCurrentUserRedacted(ctx, client, GetCurrentUserInput{}) },
		},
		{
			name:     "create_user",
			plain:    func() (*models.User, error) { return CreateUser(ctx, client, CreateUserInput{Login: "alice"}) },
			redacted: func() (*models.User, error) { return CreateUserRedacted(ctx, client, CreateUserInput{Login: "alice"}) },
		},
	}
	for _, tt := range single {
		t.Run(tt.name, func(t *testing.T) {
			u, err := tt.plain()
			if err != nil {
				t.Fatal(err)
			}
			if u.APIKey != key {
				t.Errorf("CLI variant APIKey = %q, want %q", u.APIKey, key)
			}
			u, err = tt.redacted()
			if err != nil {
				t.Fatal(err)
			}
			if u.APIKey != "" || u.Login != "alice" {
				t.Errorf("redacted variant = %+v, want login kept and APIKey empty", u)
			}
		})
	}

	t.Run("list_users", func(t *testing.T) {
		res, err := ListUsers(ctx, client, ListUsersInput{})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Users) != 2 || res.Users[0].APIKey != key {
			t.Fatalf("CLI variant = %+v, want api_key kept", res.Users)
		}
		res, err = ListUsersRedacted(ctx, client, ListUsersInput{})
		if err != nil {
			t.Fatal(err)
		}
		for i, u := range res.Users {
			if u.APIKey != "" {
				t.Errorf("Users[%d].APIKey = %q, want empty", i, u.APIKey)
			}
		}
	})
}

func TestRedactUser_Nil(t *testing.T) {
	if RedactUser(nil) != nil {
		t.Error("RedactUser(nil) should return nil")
	}
}
