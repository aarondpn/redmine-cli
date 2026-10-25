package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type GetMyAccountInput struct{}

type UpdateMyAccountInput struct {
	FirstName        *string `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName         *string `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail             *string `json:"mail,omitempty" jsonschema:"New email address."`
	MailNotification *string `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'only_my_events', 'only_assigned', 'only_owner', or 'none'."`
}

// GetMyAccount fetches /my/account.json, api_key included. The CLI calls it
// directly (`redmine my-account get`); the get_my_account MCP tool goes
// through GetMyAccountRedacted so the key never reaches an MCP client.
func GetMyAccount(ctx context.Context, client *api.Client, _ GetMyAccountInput) (*models.User, error) {
	return client.MyAccount.Get(ctx)
}

//mcpgen:tool get_my_account
//mcpgen:description Fetch the authenticated user's account via /my/account.json. Includes custom_fields.
//mcpgen:category my_account
func GetMyAccountRedacted(ctx context.Context, client *api.Client, input GetMyAccountInput) (*models.User, error) {
	user, err := GetMyAccount(ctx, client, input)
	return RedactUser(user), err
}

//mcpgen:tool update_my_account
//mcpgen:description Update the authenticated user's own account. Works without admin privileges. Requires --enable-writes.
//mcpgen:category my_account
//mcpgen:writes
func UpdateMyAccount(ctx context.Context, client *api.Client, input UpdateMyAccountInput) (MessageResult, error) {
	if err := client.MyAccount.Update(ctx, models.MyAccountUpdate{
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		Mail:             input.Mail,
		MailNotification: input.MailNotification,
	}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: "Updated my account"}, nil
}
