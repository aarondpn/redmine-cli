package ops

import (
	"context"
	"testing"
)

func TestUpdateAttachment_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]UpdateAttachmentInput{
		"no id":          {Filename: ptr("a.txt")},
		"no fields":      {ID: 1},
		"empty filename": {ID: 1, Filename: ptr(" ")},
	} {
		if _, err := UpdateAttachment(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestUpdateAttachment_Sends(t *testing.T) {
	client, calls := opsServer(t, nil)
	res, err := UpdateAttachment(context.Background(), client, UpdateAttachmentInput{ID: 7, Description: ptr(""), ContentType: ptr("text/plain")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Updated attachment 7" {
		t.Errorf("message = %q", res.Message)
	}
	c := (*calls)[0]
	if c.method != "PUT" || c.uri != "/attachments/7.json" || c.body != `{"attachment":{"description":"","content_type":"text/plain"}}` {
		t.Errorf("call = %+v", c)
	}
}

func TestDeleteAttachment(t *testing.T) {
	client, calls := opsServer(t, nil)
	if _, err := DeleteAttachment(context.Background(), client, DeleteAttachmentInput{}); err == nil {
		t.Error("expected error for id 0")
	}
	res, err := DeleteAttachment(context.Background(), client, DeleteAttachmentInput{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Deleted attachment 7" {
		t.Errorf("message = %q", res.Message)
	}
	if len(*calls) != 1 || (*calls)[0].method != "DELETE" || (*calls)[0].uri != "/attachments/7.json" {
		t.Errorf("calls = %v", *calls)
	}
}
