package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The fixtures below mirror Redmine's *.api.rsb views. Keys that only newer
// Redmine versions (or callers with more permissions) receive are absent from
// the "42" variants, because a missing key must stay missing in our output.

const membersIndex70 = `{
  "memberships": [
    {"id": 1, "project": {"id": 1, "name": "Demo"}, "user": {"id": 5, "name": "Ann Dev"},
     "roles": [{"id": 3, "name": "Manager"}, {"id": 4, "name": "Developer", "inherited": true}]},
    {"id": 2, "project": {"id": 1, "name": "Demo"}, "group": {"id": 11, "name": "Devs"},
     "roles": [{"id": 4, "name": "Developer"}]},
    {"id": 3, "project": {"id": 1, "name": "Demo"}, "group_anonymous": {"id": 13, "name": "Anonymous users"},
     "roles": [{"id": 6, "name": "Anonymous"}]},
    {"id": 4, "project": {"id": 1, "name": "Demo"}, "group_non_member": {"id": 12, "name": "Non member users"},
     "roles": [{"id": 5, "name": "Non member"}]}
  ],
  "total_count": 4, "offset": 0, "limit": 25
}`

func TestMembership_RolesKeepInheritedAndBuiltinPrincipals(t *testing.T) {
	var resp struct {
		Memberships []Membership `json:"memberships"`
	}
	if err := json.Unmarshal([]byte(membersIndex70), &resp); err != nil {
		t.Fatal(err)
	}
	ms := resp.Memberships
	if len(ms) != 4 {
		t.Fatalf("got %d memberships, want 4", len(ms))
	}
	if ms[0].User == nil || ms[0].User.Name != "Ann Dev" {
		t.Errorf("user = %+v", ms[0].User)
	}
	if ms[0].Roles[0].Inherited || !ms[0].Roles[1].Inherited {
		t.Errorf("inherited flags = %+v", ms[0].Roles)
	}
	if ms[1].Group == nil || ms[1].Group.ID != 11 {
		t.Errorf("group = %+v", ms[1].Group)
	}
	if ms[2].GroupAnonymous == nil || ms[2].GroupAnonymous.ID != 13 || ms[2].User != nil || ms[2].Group != nil {
		t.Errorf("group_anonymous = %+v", ms[2])
	}
	if ms[3].GroupNonMember == nil || ms[3].GroupNonMember.ID != 12 {
		t.Errorf("group_non_member = %+v", ms[3])
	}

	// Re-encoding keeps exactly the keys Redmine sent.
	var raw struct {
		Memberships []map[string]any `json:"memberships"`
	}
	if err := json.Unmarshal([]byte(membersIndex70), &raw); err != nil {
		t.Fatal(err)
	}
	for i, m := range ms {
		if got := encodeToMap(t, m); !reflect.DeepEqual(got, raw.Memberships[i]) {
			t.Errorf("membership %d round trip:\n got %v\nwant %v", i, got, raw.Memberships[i])
		}
	}
}

func TestMembership_InheritedFalseIsNotInvented(t *testing.T) {
	m := Membership{}
	if err := json.Unmarshal([]byte(`{"id":1,"project":{"id":1,"name":"P"},"user":{"id":2,"name":"U"},"roles":[{"id":3,"name":"R"}]}`), &m); err != nil {
		t.Fatal(err)
	}
	role := encodeToMap(t, m)["roles"].([]any)[0].(map[string]any)
	if _, ok := role["inherited"]; ok {
		t.Errorf("inherited present: %v", role)
	}
}

const userShow70 = `{
  "id": 5, "login": "ann", "admin": false, "firstname": "Ann", "lastname": "Dev",
  "mail": "ann@example.com", "created_on": "2026-01-01T00:00:00Z", "updated_on": "2026-01-02T00:00:00Z",
  "last_login_on": "2026-02-01T00:00:00Z", "passwd_changed_on": "2026-01-03T00:00:00Z",
  "avatar_url": "https://www.gravatar.com/avatar/abc?rating=PG&size=64&default=",
  "twofa_scheme": "totp", "status": 1,
  "auth_source": {"id": 2, "name": "LDAP"},
  "memberships": [
    {"id": 1, "project": {"id": 1, "name": "Demo"},
     "roles": [{"id": 3, "name": "Manager"}, {"id": 4, "name": "Developer", "inherited": true}]}
  ],
  "groups": [{"id": 11, "name": "Devs"}],
  "custom_fields": [{"id": 1, "name": "Phone", "value": "1"}]
}`

// users/show.api.rsb on 4.2: passwd_changed_on and twofa_scheme exist, but
// avatar_url (5.0+) and auth_source (6.0+) do not. passwd_changed_on is null
// for a user that never changed the password.
const userShow42 = `{
  "id": 5, "login": "ann", "admin": false, "firstname": "Ann", "lastname": "Dev",
  "mail": "ann@example.com", "created_on": "2026-01-01T00:00:00Z",
  "last_login_on": null, "passwd_changed_on": null, "twofa_scheme": null, "status": 1
}`

func TestUser_Redmine70DecodesNewFields(t *testing.T) {
	var u User
	if err := json.Unmarshal([]byte(userShow70), &u); err != nil {
		t.Fatal(err)
	}
	if u.PasswdChangedOn != "2026-01-03T00:00:00Z" || u.TwofaScheme != "totp" {
		t.Errorf("passwd_changed_on=%q twofa_scheme=%q", u.PasswdChangedOn, u.TwofaScheme)
	}
	if u.AvatarURL == "" {
		t.Error("avatar_url dropped")
	}
	if u.AuthSource == nil || u.AuthSource.ID != 2 || u.AuthSource.Name != "LDAP" {
		t.Errorf("auth_source = %+v", u.AuthSource)
	}
	if !u.Memberships[0].Roles[1].Inherited {
		t.Errorf("inherited dropped: %+v", u.Memberships[0].Roles)
	}

	var want map[string]any
	if err := json.Unmarshal([]byte(userShow70), &want); err != nil {
		t.Fatal(err)
	}
	if got := encodeToMap(t, u); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost keys:\n got %v\nwant %v", got, want)
	}
}

func TestUser_Redmine42KeepsGatedKeysAbsent(t *testing.T) {
	var u User
	keys := roundTrip(t, userShow42, &u)
	for _, k := range []string{"avatar_url", "auth_source", "twofa_scheme", "passwd_changed_on", "last_login_on", "memberships", "groups"} {
		assertAbsent(t, keys, k)
	}
	if u.AuthSource != nil {
		t.Errorf("auth_source = %+v, want nil", u.AuthSource)
	}
}

func TestGroup_BuiltinAndCustomFields(t *testing.T) {
	// groups/show.api.rsb for a regular group with memberships included.
	const regular = `{
	  "id": 11, "name": "Devs",
	  "custom_fields": [{"id": 9, "name": "Cost center", "value": "42"}],
	  "users": [{"id": 5, "name": "Ann Dev"}],
	  "memberships": [{"id": 1, "project": {"id": 1, "name": "Demo"}, "roles": [{"id": 4, "name": "Developer"}]}]
	}`
	var g Group
	if err := json.Unmarshal([]byte(regular), &g); err != nil {
		t.Fatal(err)
	}
	if g.Builtin != "" || len(g.CustomFields) != 1 {
		t.Errorf("builtin=%q custom_fields=%v", g.Builtin, g.CustomFields)
	}
	if len(g.Memberships) != 1 || g.Memberships[0].Project.Name != "Demo" || g.Memberships[0].Roles[0].Name != "Developer" {
		t.Errorf("memberships = %+v", g.Memberships)
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(regular), &want); err != nil {
		t.Fatal(err)
	}
	if got := encodeToMap(t, g); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip:\n got %v\nwant %v", got, want)
	}

	// Built-in groups carry builtin and no users array.
	var anon Group
	keys := roundTrip(t, `{"id":13,"name":"Anonymous users","builtin":"anonymous"}`, &anon)
	if anon.Builtin != "anonymous" {
		t.Errorf("builtin = %q", anon.Builtin)
	}
	assertAbsent(t, keys, "users")
	assertAbsent(t, keys, "custom_fields")
}

func TestVersion_HoursAreShowOnlyAndKeepZero(t *testing.T) {
	// versions/show.api.rsb with view_time_entries.
	const show = `{"id":2,"project":{"id":1,"name":"Demo"},"name":"1.0","description":"","status":"open",
	  "sharing":"none","created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-01T00:00:00Z",
	  "estimated_hours":0.0,"spent_hours":12.5,
	  "custom_fields":[{"id":3,"name":"Owner","value":"ann"}]}`
	var v Version
	if err := json.Unmarshal([]byte(show), &v); err != nil {
		t.Fatal(err)
	}
	if v.EstimatedHours == nil || *v.EstimatedHours != 0 {
		t.Errorf("estimated_hours = %v, want non-nil 0", v.EstimatedHours)
	}
	if v.SpentHours == nil || *v.SpentHours != 12.5 {
		t.Errorf("spent_hours = %v", v.SpentHours)
	}
	m := encodeToMap(t, v)
	if m["estimated_hours"] != float64(0) || m["spent_hours"] != 12.5 {
		t.Errorf("hours dropped: %v", m)
	}
	if len(m["custom_fields"].([]any)) != 1 {
		t.Errorf("custom_fields = %v", m["custom_fields"])
	}

	// versions/index.api.rsb and callers without view_time_entries: no hours.
	var idx Version
	keys := roundTrip(t, `{"id":2,"project":{"id":1,"name":"Demo"},"name":"1.0","description":"","status":"open","sharing":"none","created_on":"x","updated_on":"x"}`, &idx)
	assertAbsent(t, keys, "estimated_hours")
	assertAbsent(t, keys, "spent_hours")
	assertAbsent(t, keys, "custom_fields")
}

func TestTimeEntry_CustomFields(t *testing.T) {
	var te TimeEntry
	keys := roundTrip(t, `{"id":1,"project":{"id":1,"name":"P"},"user":{"id":1,"name":"U"},"activity":{"id":9,"name":"Dev"},
	  "hours":1.5,"comments":"","spent_on":"2026-01-01","created_on":"x","updated_on":"x",
	  "custom_fields":[{"id":4,"name":"Billable","value":"1"}]}`, &te)
	if len(te.CustomFields) != 1 || te.CustomFields[0].Name != "Billable" {
		t.Errorf("custom_fields = %+v", te.CustomFields)
	}
	if _, ok := keys["custom_fields"]; !ok {
		t.Error("custom_fields dropped on output")
	}
	var none TimeEntry
	assertAbsent(t, roundTrip(t, `{"id":1,"project":{"id":1,"name":"P"},"user":{"id":1,"name":"U"},"activity":{"id":9,"name":"Dev"},"hours":1}`, &none), "custom_fields")
}

func TestEnumeration_CustomFields(t *testing.T) {
	var e Enumeration
	keys := roundTrip(t, `{"id":9,"name":"Dev","is_default":false,"active":true,"custom_fields":[{"id":5,"name":"Rate","value":"80"}]}`, &e)
	if len(e.CustomFields) != 1 {
		t.Errorf("custom_fields = %+v", e.CustomFields)
	}
	if _, ok := keys["custom_fields"]; !ok {
		t.Error("custom_fields dropped on output")
	}
	var none Enumeration
	assertAbsent(t, roundTrip(t, `{"id":9,"name":"Dev","is_default":false,"active":true}`, &none), "custom_fields")
}

func TestIssueStatus_DescriptionIsVersionGated(t *testing.T) {
	// 5.1+: description is always sent, possibly empty.
	var s51 IssueStatus
	keys := roundTrip(t, `{"id":1,"name":"New","is_closed":false,"description":""}`, &s51)
	if s51.Description == nil || *s51.Description != "" {
		t.Fatalf("description = %v, want non-nil empty", s51.Description)
	}
	if string(keys["description"]) != `""` {
		t.Errorf("description = %s, want \"\"", keys["description"])
	}

	var s51d IssueStatus
	if err := json.Unmarshal([]byte(`{"id":1,"name":"New","is_closed":false,"description":"fresh"}`), &s51d); err != nil {
		t.Fatal(err)
	}
	if s51d.Description == nil || *s51d.Description != "fresh" {
		t.Errorf("description = %v", s51d.Description)
	}

	// 4.2 and allowed_statuses entries: key absent.
	var s42 IssueStatus
	assertAbsent(t, roundTrip(t, `{"id":1,"name":"New","is_closed":false}`, &s42), "description")
	if s42.Description != nil {
		t.Errorf("description = %v, want nil", s42.Description)
	}
}

func TestSearchResponse_OffsetLimitAreOptional(t *testing.T) {
	var with SearchResponse
	if err := json.Unmarshal([]byte(`{"results":[],"total_count":0,"offset":0,"limit":25}`), &with); err != nil {
		t.Fatal(err)
	}
	if with.Offset == nil || *with.Offset != 0 || with.Limit == nil || *with.Limit != 25 {
		t.Errorf("offset=%v limit=%v", with.Offset, with.Limit)
	}
	var without SearchResponse
	keys := roundTrip(t, `{"results":[],"total_count":0}`, &without)
	assertAbsent(t, keys, "offset")
	assertAbsent(t, keys, "limit")
}

func TestNormalizeCollections_CustomFieldsVersionTimeEntryGroupEnumeration(t *testing.T) {
	v, te, g, e := Version{}, TimeEntry{}, Group{}, Enumeration{}
	v.NormalizeCollections()
	te.NormalizeCollections()
	g.NormalizeCollections()
	e.NormalizeCollections()
	for name, got := range map[string]bool{
		"version":     v.CustomFields != nil && len(v.CustomFields) == 0,
		"time entry":  te.CustomFields != nil && len(te.CustomFields) == 0,
		"group":       g.CustomFields != nil && len(g.CustomFields) == 0,
		"enumeration": e.CustomFields != nil && len(e.CustomFields) == 0,
	} {
		if !got {
			t.Errorf("%s: custom_fields not normalized to []", name)
		}
	}
	// Show-only and include-driven keys stay absent.
	keys := roundTrip(t, `{"id":1}`, &v)
	assertEmptyArray(t, keys, "custom_fields")
	assertAbsent(t, keys, "estimated_hours")
	gkeys := roundTrip(t, `{"id":1}`, &g)
	assertAbsent(t, gkeys, "users")
	assertAbsent(t, gkeys, "memberships")
}
