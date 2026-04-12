package auth

import "fmt"

const (
	noProfilesConfiguredMessage = "No profiles configured. Run 'redmine auth login' to add one."
	noActiveProfileMessage      = "No active profile selected. Run 'redmine auth switch' to select one."
)

func profileNotFoundError(name string) error {
	return fmt.Errorf("profile %q not found. Run 'redmine auth list' to see available profiles", name)
}
