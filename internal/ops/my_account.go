package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type GetMyAccountInput struct{}

type UpdateMyAccountInput struct {
	FirstName          *string                  `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName           *string                  `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail               *string                  `json:"mail,omitempty" jsonschema:"New email address."`
	Language           *string                  `json:"language,omitempty" jsonschema:"Interface language code such as 'en', 'de' or 'pt-BR'; an empty string resets it to the server default. Write-only: no API response returns it."`
	MailNotification   *string                  `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'selected', 'only_my_events', 'only_assigned', 'only_owner', or 'none'. 'selected' notifies about the projects in notified_project_ids."`
	NotifiedProjectIDs *[]int                   `json:"notified_project_ids,omitempty" jsonschema:"Numeric IDs of the projects to get all notifications for when mail_notification is 'selected'. Only projects you are a member of count. Replaces the current set; an empty list clears it."`
	Pref               models.UserPreferences   `json:"pref,omitempty" jsonschema:"User preferences, sent as Redmine's top-level pref object: preference name to string value. Common keys: hide_mail ('1' hides the email address from other users, '0' shows it), time_zone (a Rails time zone name such as 'Berlin' or 'UTC'; '' uses the server default), comments_sorting ('asc' or 'desc'), no_self_notified ('1' skips notifications about your own changes, '0' sends them), warn_on_leaving_unsaved ('1' or '0'). Newer Redmine versions add keys such as auto_watch_on or default_issue_query (5.0+); Redmine silently ignores keys it does not know."`
	CustomFieldValues  models.CustomFieldValues `json:"custom_field_values,omitempty" jsonschema:"Custom field values for your own account keyed by numeric custom field ID (see custom_fields in get_my_account). Redmine only applies fields you are allowed to edit. Use a string for one value or an array of strings for a multi-value field; an empty string clears the field."`
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
		FirstName:          input.FirstName,
		LastName:           input.LastName,
		Mail:               input.Mail,
		Language:           input.Language,
		MailNotification:   input.MailNotification,
		NotifiedProjectIDs: input.NotifiedProjectIDs,
		CustomFieldValues:  input.CustomFieldValues,
		Pref:               input.Pref,
	}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: "Updated my account"}, nil
}
