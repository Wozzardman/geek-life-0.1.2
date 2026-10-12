package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ajaxray/geek-life/model"
)

type quickTaskSpec struct {
	Title      string
	Tags       []string
	Priority   int
	DueDate    int64
	Recurrence string
	Pinned     bool
}

func parseQuickTaskInput(input string, now time.Time) (quickTaskSpec, error) {
	spec := quickTaskSpec{}
	var titleParts []string
	for _, token := range strings.Fields(strings.TrimSpace(input)) {
		lower := strings.ToLower(token)
		switch {
		case strings.HasPrefix(lower, "#") && len(lower) > 1:
			spec.Tags = append(spec.Tags, strings.TrimPrefix(lower, "#"))
		case strings.HasPrefix(lower, "!"):
			priority, ok := parsePriority(strings.TrimPrefix(lower, "!"))
			if !ok {
				titleParts = append(titleParts, token)
				continue
			}
			spec.Priority = priority
		case strings.HasPrefix(lower, "@"):
			due, ok := parseQuickDueDate(strings.TrimPrefix(lower, "@"), now)
			if !ok {
				titleParts = append(titleParts, token)
				continue
			}
			spec.DueDate = due.Unix()
		case strings.HasPrefix(lower, "~") && validRecurrence(strings.TrimPrefix(lower, "~")):
			spec.Recurrence = strings.TrimPrefix(lower, "~")
		case lower == "+pin" || lower == "+pinned":
			spec.Pinned = true
		default:
			titleParts = append(titleParts, token)
		}
	}

	spec.Title = strings.TrimSpace(strings.Join(titleParts, " "))
	spec.Tags = parseTags(strings.Join(spec.Tags, " "))
	if len(spec.Title) < 3 {
		return spec, fmt.Errorf("task title should be at least 3 characters")
	}
	return spec, nil
}

func parseQuickDueDate(value string, now time.Time) (time.Time, bool) {
	today := toDate(now)
	switch value {
	case "today":
		return today, true
	case "tomorrow", "tmr":
		return today.AddDate(0, 0, 1), true
	case "week", "nextweek":
		return today.AddDate(0, 0, 7), true
	}
	parsed, err := time.ParseInLocation(dateLayoutISO, value, time.Local)
	return parsed, err == nil
}

func parsePriority(value string) (int, bool) {
	switch value {
	case "0", "none":
		return 0, true
	case "1", "low":
		return 1, true
	case "2", "medium", "med":
		return 2, true
	case "3", "high", "urgent":
		return 3, true
	default:
		return 0, false
	}
}

func priorityLabel(priority int) string {
	switch priority {
	case 1:
		return "Low"
	case 2:
		return "Medium"
	case 3:
		return "High"
	default:
		return "None"
	}
}

func parseTags(input string) []string {
	seen := make(map[string]bool)
	var tags []string
	for _, value := range strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		tag := strings.Trim(strings.TrimSpace(value), "#")
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func validRecurrence(value string) bool {
	switch value {
	case "daily", "weekly", "monthly", "yearly":
		return true
	default:
		return false
	}
}

func nextRecurringDue(task model.Task, now time.Time) int64 {
	base := toDate(now)
	if task.DueDate != 0 {
		base = toDate(time.Unix(task.DueDate, 0))
	}
	advance := func(value time.Time) time.Time {
		switch task.Recurrence {
		case "daily":
			return value.AddDate(0, 0, 1)
		case "weekly":
			return value.AddDate(0, 0, 7)
		case "monthly":
			return value.AddDate(0, 1, 0)
		case "yearly":
			return value.AddDate(1, 0, 0)
		default:
			return value
		}
	}

	next := advance(base)
	today := toDate(now)
	for validRecurrence(task.Recurrence) && !next.After(today) {
		next = advance(next)
	}
	return next.Unix()
}

func taskMatchesQuery(task model.Task, projectTitle, query string, now time.Time) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return true
	}
	today := toDate(now).Unix()
	haystack := strings.ToLower(strings.Join([]string{task.Title, task.Details, strings.Join(task.Tags, " "), projectTitle}, " "))

	for _, token := range strings.Fields(query) {
		switch {
		case strings.HasPrefix(token, "#"):
			wanted := strings.TrimPrefix(token, "#")
			matched := false
			for _, tag := range task.Tags {
				if strings.Contains(strings.ToLower(tag), wanted) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case strings.HasPrefix(token, "@"):
			if !strings.Contains(strings.ToLower(projectTitle), strings.TrimPrefix(token, "@")) {
				return false
			}
		case strings.HasPrefix(token, "!"):
			priority, ok := parsePriority(strings.TrimPrefix(token, "!"))
			if !ok || task.Priority < priority {
				return false
			}
		case strings.HasPrefix(token, "is:"):
			value := strings.TrimPrefix(token, "is:")
			if value == "done" && !task.Completed || value == "open" && task.Completed || value == "pinned" && !task.Pinned || value == "recurring" && !validRecurrence(task.Recurrence) {
				return false
			}
		case strings.HasPrefix(token, "due:"):
			value := strings.TrimPrefix(token, "due:")
			switch value {
			case "today":
				if task.DueDate != today {
					return false
				}
			case "overdue":
				if task.DueDate == 0 || task.DueDate >= today || task.Completed {
					return false
				}
			case "none", "unscheduled":
				if task.DueDate != 0 {
					return false
				}
			default:
				due, ok := parseQuickDueDate(value, now)
				if !ok || task.DueDate != due.Unix() {
					return false
				}
			}
		default:
			if !strings.Contains(haystack, token) {
				return false
			}
		}
	}
	return true
}
