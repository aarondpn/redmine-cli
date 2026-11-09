package resolver

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

func TestResolveGroupNames_FetchesOnceAndSkipsBlanks(t *testing.T) {
	calls := 0
	client, closeTS := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/groups.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"groups":[{"id":4,"name":"Developers"},{"id":5,"name":"QA"}],"total_count":2}`))
	}))
	defer closeTS()

	ids, err := ResolveGroupNames(context.Background(), client, []string{"developers", "", "9", "QA"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []int{4, 9, 5}) || calls != 1 {
		t.Errorf("ids = %v (calls %d), want [4 9 5] with one fetch", ids, calls)
	}

	ids, err = ResolveGroupNames(context.Background(), client, []string{""})
	if err != nil || ids == nil || len(ids) != 0 {
		t.Errorf("blank only: ids = %#v, %v; want an empty, non-nil slice", ids, err)
	}

	if _, err := ResolveGroupNames(context.Background(), client, []string{"Nope"}); err == nil {
		t.Error("unknown group name must fail")
	}
}
