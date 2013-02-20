package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dustin/go-jsonpointer"
)

func TestAPIBugMarshaling(t *testing.T) {
	now := time.Now()

	bug := Bug{
		Id:            "bug-1",
		Type:          "bug",
		Parent:        "bug-0",
		Title:         "a bug title",
		Description:   "a bug description",
		Status:        "new",
		Creator:       "dustin@spy.net",
		Owner:         "aaron@crate.im",
		Tags:          []string{"a", "b"},
		CreatedAt:     now,
		ModifiedAt:    now,
		ModType:       "new",
		ModBy:         "marty.schoch@gmail.com",
		Subscribers:   []string{"dustin@spy.net", "aaron@crate.im"},
		AlsoVisibleTo: []string{"user@example.com"},
		Private:       true,
	}

	j, err := json.Marshal(APIBug(bug))
	if err != nil {
		t.Fatalf("Error marshaling: %v", err)
	}

	m := map[string]interface{}{}
	err = json.Unmarshal(j, &m)
	if err != nil {
		t.Fatalf("Error unmarshaling json: %v", err)
	}

	checks := map[string]interface{}{
		"/id":                      bug.Id,
		"/type":                    bug.Type,
		"/parent":                  bug.Parent,
		"/title":                   bug.Title,
		"/description":             bug.Description,
		"/status":                  bug.Status,
		"/creator/md5":             md5string(bug.Creator),
		"/creator/email":           Email(bug.Creator).shortEmail(),
		"/owner/md5":               md5string(bug.Owner),
		"/owner/email":             Email(bug.Owner).shortEmail(),
		"/tags/0":                  "a",
		"/tags/1":                  "b",
		"/created_at":              bug.CreatedAt.Format(time.RFC3339Nano),
		"/modified_at":             bug.ModifiedAt.Format(time.RFC3339Nano),
		"/modify_type":             bug.ModType,
		"/modified_by/md5":         md5string(bug.ModBy),
		"/modified_by/email":       Email(bug.ModBy).shortEmail(),
		"/subscribers/0/md5":       md5string("dustin@spy.net"),
		"/subscribers/0/email":     "dustin",
		"/subscribers/1/md5":       md5string("aaron@crate.im"),
		"/subscribers/1/email":     "aaron",
		"/also_visible_to/0/md5":   md5string("user@example.com"),
		"/also_visible_to/0/email": "user",
		"/private":                 bug.Private,
	}

	for p, v := range checks {
		got := jsonpointer.Get(m, p)
		if got != v {
			t.Errorf("Error at %v, expected %v, got %v",
				p, v, got)
		}
	}
}
