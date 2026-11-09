package user

import (
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func rowsText(u *models.User) string {
	var b strings.Builder
	for _, kv := range userDetailRows(u) {
		b.WriteString(kv.Key + "=" + kv.Value + "\n")
	}
	return b.String()
}

func TestUserDetailRows_AccountDates(t *testing.T) {
	got := rowsText(&models.User{ID: 1, CreatedOn: "c", UpdatedOn: "u", PasswdChangedOn: "p", TwofaScheme: "totp"})
	for _, want := range []string{"Updated=u\n", "Password Changed=p\n", "Two-factor=totp\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	got = rowsText(&models.User{ID: 1, CreatedOn: "c"})
	for _, unwanted := range []string{"Updated", "Password", "Two-factor"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, got)
		}
	}
}
