package cmdutil_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func newCustomFieldCompletionServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"custom_fields":[
{"id":1,"name":"Severity","customized_type":"issue","field_format":"list","is_required":true,"possible_values":[{"value":"Low"},{"value":"High"}]},
{"id":2,"name":"Severity","customized_type":"time_entry","field_format":"string"}]}`))
	}))
}

func TestCompleteCustomFieldValues_Names(t *testing.T) {
	srv := newCustomFieldCompletionServer(t, http.StatusOK)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	got, directive := cmdutil.CompleteCustomFieldValues(f, "issue")(&cobra.Command{}, nil, "Se")
	if want := []string{"Severity=\tlist, required"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("completions = %q, want %q", got, want)
	}
	if directive&cobra.ShellCompDirectiveNoSpace == 0 {
		t.Errorf("directive %v lacks NoSpace", directive)
	}
}

func TestCompleteCustomFieldValues_PossibleValues(t *testing.T) {
	srv := newCustomFieldCompletionServer(t, http.StatusOK)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	got, directive := cmdutil.CompleteCustomFieldValues(f, "issue")(&cobra.Command{}, nil, "Severity=")
	if want := []string{"Severity=Low", "Severity=High"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("completions = %q, want %q", got, want)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v", directive)
	}
}

func TestCompleteCustomFieldValues_ForbiddenYieldsNothing(t *testing.T) {
	srv := newCustomFieldCompletionServer(t, http.StatusForbidden)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	got, _ := cmdutil.CompleteCustomFieldValues(f, "issue")(&cobra.Command{}, nil, "")
	if len(got) != 0 {
		t.Errorf("completions = %q, want none", got)
	}
}
