package user

import (
	"strings"
	"testing"
)

func TestParsePrefs_RejectsTrueFalse(t *testing.T) {
	for raw, hint := range map[string]string{"hide_mail=true": "hide_mail=1", "no_self_notified=FALSE": "no_self_notified=0"} {
		if _, err := ParsePrefs([]string{raw}); err == nil || !strings.Contains(err.Error(), "use "+hint) {
			t.Errorf("%s: err = %v, want a hint to %s", raw, err, hint)
		}
	}
	got, err := ParsePrefs([]string{"hide_mail=1", "time_zone=", "comments_sorting=desc"})
	if err != nil || got["hide_mail"] != "1" || got["time_zone"] != "" || got["comments_sorting"] != "desc" {
		t.Errorf("valid prefs: %v, %v", got, err)
	}
}

func TestValidateMailNotification_OnlyMyWatches(t *testing.T) {
	if err := ValidateMailNotification("only_my_watches"); err != nil {
		t.Errorf("only_my_watches: %v", err)
	}
}
