package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/ajaxray/geek-life/model"
)

func TestParseQuickTaskInput(t *testing.T) {
	now := time.Date(2026, time.August, 14, 16, 30, 0, 0, time.Local)
	spec, err := parseQuickTaskInput("Ship release #Work #urgent !high @tomorrow ~weekly +pin", now)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Title != "Ship release" {
		t.Fatalf("unexpected title: %q", spec.Title)
	}
	if !reflect.DeepEqual(spec.Tags, []string{"urgent", "work"}) {
		t.Fatalf("unexpected tags: %#v", spec.Tags)
	}
	if spec.Priority != 3 || !spec.Pinned || spec.Recurrence != "weekly" {
		t.Fatalf("metadata was not parsed: %#v", spec)
	}
	wantDue := time.Date(2026, time.August, 15, 0, 0, 0, 0, time.Local).Unix()
	if spec.DueDate != wantDue {
		t.Fatalf("due date = %d, want %d", spec.DueDate, wantDue)
	}
}

func TestParseQuickTaskInputPreservesUnknownTokens(t *testing.T) {
	spec, err := parseQuickTaskInput("Discuss @alex !important ~sometimes", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if spec.Title != "Discuss @alex !important ~sometimes" {
		t.Fatalf("unknown metadata-like tokens should remain in the title: %q", spec.Title)
	}
}

func TestStructuredTaskQuery(t *testing.T) {
	now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.Local)
	task := model.Task{
		Title:     "Ship terminal release",
		Details:   "Validate the Windows installer",
		Tags:      []string{"work", "release"},
		Priority:  3,
		Pinned:    true,
		DueDate:   toDate(now).Unix(),
		Completed: false,
	}
	cases := []struct {
		query string
		want  bool
	}{
		{"terminal #work @geek !2 is:open due:today", true},
		{"#personal", false},
		{"!3 is:pinned", true},
		{"due:2026-08-14", true},
		{"is:done", false},
		{"due:none", false},
		{"installer @other", false},
	}
	for _, tc := range cases {
		if got := taskMatchesQuery(task, "Geek Life", tc.query, now); got != tc.want {
			t.Errorf("query %q = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestNextRecurringDueSkipsPastOccurrences(t *testing.T) {
	now := time.Date(2026, time.August, 14, 9, 0, 0, 0, time.Local)
	task := model.Task{
		DueDate:    time.Date(2026, time.July, 31, 0, 0, 0, 0, time.Local).Unix(),
		Recurrence: "weekly",
	}
	got := time.Unix(nextRecurringDue(task, now), 0)
	want := time.Date(2026, time.August, 21, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("next recurrence = %v, want %v", got, want)
	}
}

func TestFuzzyMatch(t *testing.T) {
	if !fuzzyMatch("Create JSON backup", "cjb") {
		t.Fatal("expected ordered fuzzy characters to match")
	}
	if fuzzyMatch("Create JSON backup", "bcj") {
		t.Fatal("out-of-order fuzzy characters should not match")
	}
}
