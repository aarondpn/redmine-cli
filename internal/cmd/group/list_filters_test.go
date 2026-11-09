package group

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestGroupList_Builtin(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"groups":[],"total_count":0}`)

	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdGroupList(f)
	cmd.SetArgs([]string{"--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if q := rec.Last(t); q.Has("builtin") {
		t.Errorf("builtin sent by default: %s", q.Encode())
	}

	f = testutil.NewFactory(t, rec.URL)
	cmd = newCmdGroupList(f)
	cmd.SetArgs([]string{"--builtin", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.WantQueryValues(t, rec.Last(t), "builtin", "1")
}
