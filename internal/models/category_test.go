package models

import (
	"encoding/json"
	"testing"
)

func TestIssueCategoryUpdate_MarshalJSON(t *testing.T) {
	name := "Backend"
	zero, seven := 0, 7
	tests := []struct {
		name string
		in   IssueCategoryUpdate
		want string
	}{
		{"empty", IssueCategoryUpdate{}, `{}`},
		{"name only", IssueCategoryUpdate{Name: &name}, `{"name":"Backend"}`},
		{"assignee set", IssueCategoryUpdate{AssignedToID: &seven}, `{"assigned_to_id":7}`},
		{"assignee cleared", IssueCategoryUpdate{AssignedToID: &zero}, `{"assigned_to_id":""}`},
		{"name and clear", IssueCategoryUpdate{Name: &name, AssignedToID: &zero}, `{"name":"Backend","assigned_to_id":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestIssueCategoryCreate_MarshalJSON(t *testing.T) {
	got, _ := json.Marshal(IssueCategoryCreate{Name: "UI"})
	if string(got) != `{"name":"UI"}` {
		t.Errorf("got %s", got)
	}
	got, _ = json.Marshal(IssueCategoryCreate{Name: "UI", AssignedToID: 3})
	if string(got) != `{"name":"UI","assigned_to_id":3}` {
		t.Errorf("got %s", got)
	}
}

func TestAttachmentUpdate_MarshalJSON(t *testing.T) {
	d := ""
	got, _ := json.Marshal(AttachmentUpdate{Description: &d})
	if string(got) != `{"description":""}` {
		t.Errorf("got %s, want an empty description to be sent so it can be cleared", got)
	}
	got, _ = json.Marshal(AttachmentUpdate{})
	if string(got) != `{}` {
		t.Errorf("got %s", got)
	}
}
