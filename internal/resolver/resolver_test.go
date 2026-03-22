package resolver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/debug"
	"github.com/aarondpn/redmine-cli/internal/models"
)

func testClient(t *testing.T) *api.Client {
	t.Helper()
	c, err := api.NewClient(&config.Config{Server: "http://localhost"}, debug.New(nil))
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}
	return c
}

// --- Resolve tests ---

func TestResolve_NumericInput(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		t.Fatal("fetcher should not be called for numeric input")
		return nil, nil
	}

	id, err := Resolve("42", "tracker", client, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

func TestResolve_ExactMatch(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return []Option{
			{ID: 1, Name: "Bug"},
			{ID: 2, Name: "Feature"},
			{ID: 3, Name: "Support"},
		}, nil
	}

	id, err := Resolve("Feature", "tracker", client, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 2 {
		t.Errorf("expected ID 2, got %d", id)
	}
}

func TestResolve_CaseInsensitiveMatch(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return []Option{
			{ID: 1, Name: "Bug"},
			{ID: 2, Name: "Feature"},
		}, nil
	}

	id, err := Resolve("feature", "tracker", client, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 2 {
		t.Errorf("expected ID 2, got %d", id)
	}
}

func TestResolve_MultipleExactMatches(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return []Option{
			{ID: 1, Name: "Bug"},
			{ID: 5, Name: "Bug"},
		}, nil
	}

	_, err := Resolve("Bug", "tracker", client, fetcher)
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
	if !strings.Contains(err.Error(), "multiple trackers match") {
		t.Errorf("expected multiple match error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ID: 1") || !strings.Contains(err.Error(), "ID: 5") {
		t.Errorf("expected both IDs in error, got: %v", err)
	}
}

func TestResolve_NoMatch_SmallList(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return []Option{
			{ID: 1, Name: "Bug"},
			{ID: 2, Name: "Feature"},
			{ID: 3, Name: "Support"},
		}, nil
	}

	_, err := Resolve("Featrue", "tracker", client, fetcher)
	if err == nil {
		t.Fatal("expected error for no match")
	}
	// Small list: should show all available options
	if !strings.Contains(err.Error(), "Available trackers:") {
		t.Errorf("expected available options list, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Bug") || !strings.Contains(err.Error(), "Feature") {
		t.Errorf("expected all options listed, got: %v", err)
	}
}

func TestResolve_NoMatch_LargeList_WithSuggestion(t *testing.T) {
	client := testClient(t)
	opts := make([]Option, 15)
	for i := range 15 {
		opts[i] = Option{ID: i + 1, Name: fmt.Sprintf("Unrelated%d", i)}
	}
	opts[7] = Option{ID: 8, Name: "Development"}

	fetcher := func() ([]Option, error) { return opts, nil }

	_, err := Resolve("Devlopment", "category", client, fetcher)
	if err == nil {
		t.Fatal("expected error for no match")
	}
	if !strings.Contains(err.Error(), `Did you mean "Development"`) {
		t.Errorf("expected fuzzy suggestion, got: %v", err)
	}
}

func TestResolve_FetcherError(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return nil, fmt.Errorf("connection refused")
	}

	_, err := Resolve("Bug", "tracker", client, fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected fetcher error propagated, got: %v", err)
	}
}

func TestResolve_ForbiddenError(t *testing.T) {
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return nil, &api.APIError{StatusCode: 403, URL: "/groups.json"}
	}

	_, err := Resolve("Developers", "group", client, fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot resolve group by name") {
		t.Errorf("expected permission hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "numeric ID") {
		t.Errorf("expected numeric ID suggestion, got: %v", err)
	}
}

func TestResolve_ForbiddenErrorWrapped(t *testing.T) {
	// Simulates the real code path: fetcher wraps the 403 with fmt.Errorf,
	// e.g. ResolveGroup's "failed to fetch groups: %w"
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return nil, fmt.Errorf("failed to fetch groups: %w", &api.APIError{StatusCode: 403, URL: "/groups.json"})
	}

	_, err := Resolve("Developers", "group", client, fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot resolve group by name") {
		t.Errorf("expected permission hint even for wrapped 403, got: %v", err)
	}
	// Should NOT contain the raw API error — we want a clean message
	if strings.Contains(err.Error(), "API error 403") {
		t.Errorf("expected clean message without raw API error, got: %v", err)
	}
}

func TestResolve_NonForbiddenAPIError(t *testing.T) {
	// Non-403 API errors (401, 500, etc.) should pass through unchanged
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		return nil, &api.APIError{StatusCode: 401, URL: "/trackers.json"}
	}

	_, err := Resolve("Bug", "tracker", client, fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "cannot resolve") {
		t.Errorf("non-403 error should not get permission hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected original error propagated, got: %v", err)
	}
}

func TestResolve_ForbiddenNumericBypass(t *testing.T) {
	// Numeric input should bypass the fetcher entirely, even if it would 403
	client := testClient(t)
	fetcher := func() ([]Option, error) {
		t.Fatal("fetcher should not be called for numeric input")
		return nil, nil
	}

	id, err := Resolve("42", "group", client, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

func TestIsForbiddenErr_WrappedError(t *testing.T) {
	inner := &api.APIError{StatusCode: 403, URL: "/users.json"}
	wrapped := fmt.Errorf("failed to fetch users: %w", inner)
	if !isForbiddenErr(wrapped) {
		t.Error("expected isForbiddenErr to detect wrapped 403")
	}
}

func TestIsForbiddenErr_NonForbidden(t *testing.T) {
	err := &api.APIError{StatusCode: 404, URL: "/users.json"}
	if isForbiddenErr(err) {
		t.Error("expected isForbiddenErr to return false for 404")
	}

	if isForbiddenErr(fmt.Errorf("some other error")) {
		t.Error("expected isForbiddenErr to return false for non-API error")
	}

	if isForbiddenErr(nil) {
		t.Error("expected isForbiddenErr to return false for nil")
	}
}

func TestResolveProjectFromList_MatchesDisplayName(t *testing.T) {
	client := testClient(t)
	projects := []models.Project{
		{ID: 129, Name: "Internal Platform", Identifier: "platform"},
		{ID: 130, Name: "Internal Tools", Identifier: "tools"},
	}

	id, identifier, err := resolveProjectFromList(client, "Internal Platform", projects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 129 || identifier != "platform" {
		t.Fatalf("expected project 129/platform, got %d/%s", id, identifier)
	}
}

func TestResolveProjectFromList_MatchesIdentifier(t *testing.T) {
	client := testClient(t)
	projects := []models.Project{
		{ID: 129, Name: "Internal Platform", Identifier: "platform"},
	}

	id, identifier, err := resolveProjectFromList(client, "platform", projects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 129 || identifier != "platform" {
		t.Fatalf("expected project 129/platform, got %d/%s", id, identifier)
	}
}

func TestResolveProjectFromList_SuggestionsIncludeIdentifier(t *testing.T) {
	client := testClient(t)
	projects := []models.Project{
		{ID: 129, Name: "Internal Platform", Identifier: "platform"},
		{ID: 130, Name: "Internal Tools", Identifier: "tools"},
	}

	_, _, err := resolveProjectFromList(client, "Internal Pltform", projects)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Internal Platform [platform]") {
		t.Fatalf("expected identifier in suggestion output, got: %v", err)
	}
}

// --- buildSuggestions tests ---

func TestBuildSuggestions_SmallListShowsAll(t *testing.T) {
	names := []string{"Bug", "Feature", "Support", "Task", "Epic"}
	ids := []int{1, 2, 3, 4, 5}

	result := buildSuggestions("Featrue", names, ids, "tracker")

	if !strings.Contains(result, "Available trackers:") {
		t.Errorf("expected all options listed for small list, got: %s", result)
	}
	for _, name := range names {
		if !strings.Contains(result, name) {
			t.Errorf("expected %q in output, got: %s", name, result)
		}
	}
}

func TestBuildSuggestions_BoundarySmallList(t *testing.T) {
	// Exactly smallListThreshold items → should show all
	names := make([]string, smallListThreshold)
	ids := make([]int, smallListThreshold)
	for i := range smallListThreshold {
		names[i] = fmt.Sprintf("Option%d", i)
		ids[i] = i + 1
	}

	result := buildSuggestions("xyz", names, ids, "tracker")
	if !strings.Contains(result, "Available trackers:") {
		t.Errorf("expected all options for list at threshold, got: %s", result)
	}
}

func TestBuildSuggestions_BoundaryLargeList(t *testing.T) {
	// One more than smallListThreshold → should use fuzzy matching
	names := make([]string, smallListThreshold+1)
	ids := make([]int, smallListThreshold+1)
	for i := range smallListThreshold + 1 {
		names[i] = fmt.Sprintf("ReallyLongUnrelatedOption%d", i)
		ids[i] = i + 1
	}

	result := buildSuggestions("xyz", names, ids, "tracker")
	if strings.Contains(result, "Available trackers:") {
		t.Errorf("expected fuzzy matching for list above threshold, got: %s", result)
	}
	if !strings.Contains(result, "No similar trackers found") {
		t.Errorf("expected no similar match message, got: %s", result)
	}
}

func TestBuildSuggestions_LargeListSingleCloseMatch(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "Unrelated" + strings.Repeat("x", i+5)
		ids[i] = i + 1
	}
	names[5] = "Development"
	ids[5] = 6

	result := buildSuggestions("Devlopment", names, ids, "category")

	if !strings.Contains(result, `Did you mean "Development" (ID: 6)?`) {
		t.Errorf("expected single suggestion for close match, got: %s", result)
	}
}

func TestBuildSuggestions_LargeListMultipleEqualDistance(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "ZZZunrelated" + strings.Repeat("z", i)
		ids[i] = i + 1
	}
	// All distance 1 from "Deve"
	names[0] = "Deva"
	names[1] = "Devs"
	names[2] = "Devx"

	result := buildSuggestions("Deve", names, ids, "tracker")

	if !strings.Contains(result, "Did you mean:") {
		t.Errorf("expected multiple suggestions, got: %s", result)
	}
	// All three should appear
	for _, name := range []string{"Deva", "Devs", "Devx"} {
		if !strings.Contains(result, name) {
			t.Errorf("expected %q in suggestions, got: %s", name, result)
		}
	}
}

func TestBuildSuggestions_LargeListNoCloseMatch(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "ReallyLongOptionName" + strings.Repeat("x", i)
		ids[i] = i + 1
	}

	result := buildSuggestions("zzzzz", names, ids, "tracker")

	if !strings.Contains(result, "No similar trackers found") {
		t.Errorf("expected no similar match message, got: %s", result)
	}
}

func TestBuildSuggestions_EmptyOptions(t *testing.T) {
	result := buildSuggestions("anything", nil, nil, "tracker")

	if !strings.Contains(result, "Available trackers:") {
		t.Errorf("expected empty available list for nil options, got: %s", result)
	}
}

func TestBuildSuggestions_MaxSuggestionsLimit(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "Ab" + strings.Repeat("c", i)
		ids[i] = i + 1
	}

	result := buildSuggestions("Ab", names, ids, "tracker")

	lines := strings.Split(result, "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "  - ") {
			count++
		}
	}
	if count > maxSuggestions {
		t.Errorf("expected at most %d suggestions, got %d in: %s", maxSuggestions, count, result)
	}
}

func TestBuildSuggestions_CaseInsensitive(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "ZZZunrelated" + strings.Repeat("z", i)
		ids[i] = i + 1
	}
	names[3] = "Feature"
	ids[3] = 4

	result := buildSuggestions("FEATUR", names, ids, "tracker")

	if !strings.Contains(result, "Feature") {
		t.Errorf("expected case-insensitive match for Feature, got: %s", result)
	}
}

func TestBuildSuggestions_PreservesOriginalNameInOutput(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "ZZZunrelated" + strings.Repeat("z", i)
		ids[i] = i + 1
	}
	names[0] = "MyFeature"
	ids[0] = 42

	result := buildSuggestions("myfeatur", names, ids, "tracker")

	// Should display original casing, not lowercased
	if !strings.Contains(result, "MyFeature") {
		t.Errorf("expected original casing preserved, got: %s", result)
	}
	if !strings.Contains(result, "ID: 42") {
		t.Errorf("expected correct ID, got: %s", result)
	}
}

func TestBuildSuggestions_InputInErrorMessage(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "ZZZunrelated" + strings.Repeat("z", i)
		ids[i] = i + 1
	}

	result := buildSuggestions("MyTypo", names, ids, "tracker")

	if !strings.Contains(result, `"MyTypo"`) {
		t.Errorf("expected input quoted in message, got: %s", result)
	}
}

func TestBuildSuggestions_ThresholdScalesWithInputLength(t *testing.T) {
	names := make([]string, 20)
	ids := make([]int, 20)
	for i := range 20 {
		names[i] = "ZZZunrelated" + strings.Repeat("z", i)
		ids[i] = i + 1
	}
	// "Administration" is 14 chars → threshold = 14/3 = 4
	// "Xdministxatixn" has 3 edits from "Administration"
	names[0] = "Administration"
	ids[0] = 99

	result := buildSuggestions("Xdministxatixn", names, ids, "tracker")

	if !strings.Contains(result, "Administration") {
		t.Errorf("expected match within scaled threshold, got: %s", result)
	}
}
