package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// SelectItem represents an item for selection.
type SelectItem struct {
	Label string
	Value string
}

// RunSelector presents a selection list and returns the chosen value.
func RunSelector(title string, items []SelectItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items to select from")
	}

	options := make([]huh.Option[string], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item.Label, item.Value)
	}

	var selected string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(options...).
				Value(&selected),
		),
	).Run()
	if err != nil {
		return "", err
	}
	return selected, nil
}
