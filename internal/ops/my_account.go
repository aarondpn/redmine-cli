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

//mcpgen:tool get_my_account
//mcpgen:description Fetch the authenticated user's account via /my/account.json. Includes api_key and custom_fields.
//mcpgen:category my_account
func GetMyAccount(ctx context.Context, client *api.Client, _ GetMyAccountInput) (*models.User, error) {
	return client.MyAccount.Get(ctx)
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
