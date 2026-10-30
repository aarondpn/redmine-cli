package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type GetMyAccountInput struct{}

type UpdateMyAccountInput struct {
	FirstName         *string                  `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName          *string                  `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail              *string                  `json:"mail,omitempty" jsonschema:"New email address."`
	MailNotification  *string                  `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'only_my_events', 'only_assigned', 'only_owner', or 'none'."`
	CustomFieldValues models.CustomFieldValues `json:"custom_field_values,omitempty" jsonschema:"Custom field values for your own account keyed by numeric custom field ID (see custom_fields in get_my_account). Redmine only applies fields you are allowed to edit. Use a string for one value or an array of strings for a multi-value field; an empty string clears the field."`
}

//mcpgen:tool get_my_account
//mcpgen:description Fetch the authenticated user's account via /my/account.json. Includes custom_fields.
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
		FirstName:         input.FirstName,
		LastName:          input.LastName,
		Mail:              input.Mail,
		MailNotification:  input.MailNotification,
		CustomFieldValues: input.CustomFieldValues,
	}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: "Updated my account"}, nil
}
