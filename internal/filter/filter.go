package filter

import (
	"strings"
	"time"
)

type Query struct {
	DateFrom, DateTo, Type, Q string
}

func EventInDateRange(startAt, dateFrom, dateTo string) bool {
	if dateFrom == "" && dateTo == "" {
		return true
	}
	if startAt == "" {
		return true
	}
	d, err := time.Parse(time.RFC3339, startAt)
	if err != nil {
		d, err = time.Parse(time.RFC3339Nano, startAt)
	}
	if err != nil {
		return true
	}
	if dateFrom != "" {
		t0, err := time.ParseInLocation("2006-01-02", dateFrom, time.Local)
		if err == nil && d.Before(t0) {
			return false
		}
	}
	if dateTo != "" {
		t1, err := time.ParseInLocation("2006-01-02", dateTo, time.Local)
		if err == nil {
			t1 = t1.Add(24*time.Hour - time.Millisecond)
			if d.After(t1) {
				return false
			}
		}
	}
	return true
}

func eventMatchesType(typeLabel, title, typ string) bool {
	if typ == "" {
		return true
	}
	hay := strings.ToLower(typeLabel + " " + title)
	switch typ {
	case "forum":
		return strings.Contains(hay, "forum") || strings.Contains(hay, "salon")
	case "job":
		return strings.Contains(hay, "job") || strings.Contains(hay, "dating")
	case "atelier":
		return strings.Contains(hay, "atelier") || strings.Contains(hay, "réunion")
	default:
		return true
	}
}

type Event struct {
	Key           string   `json:"key"`
	Title         string   `json:"title"`
	StartAt       string   `json:"startAt"`
	LocationLabel string   `json:"locationLabel"`
	URL           string   `json:"url"`
	TypeLabel     string   `json:"typeLabel"`
	Lat           *float64 `json:"lat,omitempty"`
	Lon           *float64 `json:"lon,omitempty"`
}

func FilterEvents(events []Event, q Query) []Event {
	var out []Event
	for _, ev := range events {
		if !EventInDateRange(ev.StartAt, q.DateFrom, q.DateTo) {
			continue
		}
		if !eventMatchesType(ev.TypeLabel, ev.Title, q.Type) {
			continue
		}
		if kw := strings.TrimSpace(strings.ToLower(q.Q)); kw != "" {
			blob := strings.ToLower(ev.Title + " " + ev.LocationLabel + " " + ev.TypeLabel)
			if !strings.Contains(blob, kw) {
				continue
			}
		}
		out = append(out, ev)
	}
	return out
}
