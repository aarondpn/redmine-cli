package group

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestGroupCreate_TwofaRequired(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{"POST /groups.json": `{"group":{"id":4,"name":"Ops"}}`}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupCreate(f)
	cmd.SetArgs([]string{"--name", "Ops", "--twofa-required"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{"group": map[string]any{"name": "Ops", "twofa_required": true}})
}

func TestGroupUpdate_TwofaRequired(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want map[string]any
	}{
		{[]string{"4", "--twofa-required=false"}, map[string]any{"group": map[string]any{"twofa_required": false}}},
		{[]string{"4", "--name", "Ops"}, map[string]any{"group": map[string]any{"name": "Ops"}}},
	} {
		var writes []map[string]any
		srv := testutil.RouteServer(t, map[string]string{"PUT /groups/4.json": ""}, &writes)
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdGroupUpdate(f)
		cmd.SetArgs(tt.args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		testutil.RequireOneWrite(t, writes, tt.want)
	}
}
