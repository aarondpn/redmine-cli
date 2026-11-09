package api

import (
	"context"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func writableFieldsClient(t *testing.T, status int, resp string) (*Client, *capturedRequest) {
	t.Helper()
	c, rec := captureServer(t, status, resp)
	c.Users = &UserService{client: c}
	c.MyAccount = &MyAccountService{client: c}
	c.Groups = &GroupService{client: c}
	c.Projects = &ProjectService{client: c}
	c.Versions = &VersionService{client: c}
	c.Memberships = &MembershipService{client: c}
	return c, rec
}

func ptr[T any](v T) *T { return &v }

func TestUserService_CreateSendsPrefNextToUser(t *testing.T) {
	c, rec := writableFieldsClient(t, 201, `{"user":{"id":3,"login":"jdoe"}}`)
	_, err := c.Users.Create(context.Background(), models.UserCreate{
		Login: "jdoe", FirstName: "J", LastName: "D", Mail: "j@x.y",
		Status: ptr(3), Language: ptr("de"),
		Pref: models.UserPreferences{"hide_mail": "1"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"pref":{"hide_mail":"1"},"send_information":true,"user":{"login":"jdoe","firstname":"J","lastname":"D","mail":"j@x.y","status":3,"language":"de"}}`
	if rec.method != "POST" || rec.path != "/users.json" || rec.body != want {
		t.Errorf("request = %s %s %s\nwant body %s", rec.method, rec.path, rec.body, want)
	}
}

func TestUserService_UpdateSendsNewFields(t *testing.T) {
	c, rec := writableFieldsClient(t, 204, "")
	err := c.Users.Update(context.Background(), 7, models.UserUpdate{
		Password:             ptr("s3cret pw"),
		PasswordConfirmation: ptr("s3cret pw"),
		Language:             ptr(""),
		GroupIDs:             &[]int{},
		NotifiedProjectIDs:   &[]int{4, 5},
		Pref:                 models.UserPreferences{"time_zone": "Berlin", "no_self_notified": "0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"pref":{"no_self_notified":"0","time_zone":"Berlin"},"user":{"password":"s3cret pw","password_confirmation":"s3cret pw","language":"","notified_project_ids":[4,5],"group_ids":[]}}`
	if rec.method != "PUT" || rec.path != "/users/7.json" || rec.body != want {
		t.Errorf("request = %s %s %s\nwant body %s", rec.method, rec.path, rec.body, want)
	}
}

func TestUserService_UpdateWithoutPrefOmitsIt(t *testing.T) {
	c, rec := writableFieldsClient(t, 204, "")
	if err := c.Users.Update(context.Background(), 7, models.UserUpdate{FirstName: ptr("J")}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"user":{"firstname":"J"}}` {
		t.Errorf("body = %s", rec.body)
	}
}

func TestMyAccountService_UpdateSendsNewFields(t *testing.T) {
	c, rec := writableFieldsClient(t, 204, "")
	err := c.MyAccount.Update(context.Background(), models.MyAccountUpdate{
		Language:           ptr("fr"),
		MailNotification:   ptr("selected"),
		NotifiedProjectIDs: &[]int{2},
		Pref:               models.UserPreferences{"no_self_notified": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"pref":{"no_self_notified":"1"},"user":{"language":"fr","mail_notification":"selected","notified_project_ids":[2]}}`
	if rec.method != "PUT" || rec.path != "/my/account.json" || rec.body != want {
		t.Errorf("request = %s %s %s\nwant body %s", rec.method, rec.path, rec.body, want)
	}
}

func TestGroupService_TwofaRequired(t *testing.T) {
	c, rec := writableFieldsClient(t, 201, `{"group":{"id":4,"name":"Ops"}}`)
	if _, err := c.Groups.Create(context.Background(), models.GroupCreate{Name: "Ops", TwofaRequired: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"group":{"name":"Ops","twofa_required":true}}` {
		t.Errorf("create body = %s", rec.body)
	}

	c, rec = writableFieldsClient(t, 204, "")
	if err := c.Groups.Update(context.Background(), 4, models.GroupUpdate{TwofaRequired: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"group":{"twofa_required":false}}` {
		t.Errorf("update body = %s", rec.body)
	}
}

func TestProjectService_DefaultIssueQueryID(t *testing.T) {
	c, rec := writableFieldsClient(t, 201, `{"project":{"id":1,"name":"Demo","identifier":"demo"}}`)
	if _, err := c.Projects.Create(context.Background(), models.ProjectCreate{Name: "Demo", Identifier: "demo", DefaultIssueQueryID: 12}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"project":{"name":"Demo","identifier":"demo","default_issue_query_id":12}}` {
		t.Errorf("create body = %s", rec.body)
	}

	c, rec = writableFieldsClient(t, 204, "")
	if err := c.Projects.Update(context.Background(), "demo", models.ProjectUpdate{DefaultIssueQueryID: ptr(0)}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"project":{"default_issue_query_id":0}}` {
		t.Errorf("update body = %s", rec.body)
	}
}

func TestVersionService_DefaultProjectVersion(t *testing.T) {
	c, rec := writableFieldsClient(t, 201, `{"version":{"id":9,"name":"v1"}}`)
	if _, err := c.Versions.Create(context.Background(), "demo", models.VersionCreate{Name: "v1", DefaultProjectVersion: true}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"version":{"name":"v1","default_project_version":true}}` {
		t.Errorf("create body = %s", rec.body)
	}

	c, rec = writableFieldsClient(t, 204, "")
	if err := c.Versions.Update(context.Background(), 9, models.VersionUpdate{Name: ptr("v2")}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"version":{"name":"v2"}}` {
		t.Errorf("update without the flag must not send it: %s", rec.body)
	}
}

func TestMembershipService_CreateUserIDs(t *testing.T) {
	c, rec := writableFieldsClient(t, 201, `{"membership":{"id":1,"project":{"id":1,"name":"Demo"},"user":{"id":5,"name":"A"},"roles":[]}}`)
	if _, err := c.Memberships.Create(context.Background(), "demo", models.MembershipCreate{UserIDs: []int{5, 6}, RoleIDs: []int{3}}); err != nil {
		t.Fatal(err)
	}
	if rec.path != "/projects/demo/memberships.json" || rec.body != `{"membership":{"user_ids":[5,6],"role_ids":[3]}}` {
		t.Errorf("request = %s %s", rec.path, rec.body)
	}

	if _, err := c.Memberships.Create(context.Background(), "demo", models.MembershipCreate{UserID: 5, RoleIDs: []int{3}}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"membership":{"user_id":5,"role_ids":[3]}}` {
		t.Errorf("single-user body = %s", rec.body)
	}
}
