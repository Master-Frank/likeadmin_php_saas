package biz

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronExpr struct {
	min, hour, dom, month, dow []int
	domStar, dowStar           bool
}

func ParseCron(expr string) (*CronExpr, error) {
	expr = expandCronMacros(expr)
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return nil, fmt.Errorf("定时任务运行规则错误")
	}
	min, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return nil, err
	}
	hour, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return nil, err
	}
	dom, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return nil, err
	}
	month, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return nil, err
	}
	dow, err := parseCronField(parts[4], 0, 7)
	if err != nil {
		return nil, err
	}
	norm := make([]int, 0, len(dow))
	seen := map[int]bool{}
	for _, d := range dow {
		if d == 7 {
			d = 0
		}
		if !seen[d] {
			seen[d] = true
			norm = append(norm, d)
		}
	}
	return &CronExpr{
		min: min, hour: hour, dom: dom, month: month, dow: norm,
		domStar: parts[2] == "*", dowStar: parts[4] == "*",
	}, nil
}

func ValidCron(expr string) bool {
	_, err := ParseCron(expr)
	return err == nil
}

func (e *CronExpr) Match(t time.Time) bool {
	if !containsInt(e.min, t.Minute()) || !containsInt(e.hour, t.Hour()) || !containsInt(e.month, int(t.Month())) {
		return false
	}
	domOK := containsInt(e.dom, t.Day())
	dowOK := containsInt(e.dow, int(t.Weekday()))
	if e.domStar || e.dowStar {
		return (e.domStar || domOK) && (e.dowStar || dowOK)
	}
	return domOK || dowOK
}

func (e *CronExpr) Next(from time.Time) time.Time {
	t := from.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 366*24*60; i++ {
		if e.Match(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}

func (e *CronExpr) NextN(from time.Time, n int) []time.Time {
	out := make([]time.Time, 0, n)
	cur := from
	for i := 0; i < n; i++ {
		nxt := e.Next(cur)
		if nxt.IsZero() {
			break
		}
		out = append(out, nxt)
		cur = nxt
	}
	return out
}

func CronExpressionLists(expr string) ([]map[string]any, error) {
	e, err := ParseCron(expr)
	if err != nil {
		return nil, err
	}
	runs := e.NextN(time.Now(), 5)
	out := make([]map[string]any, 0, 6)
	for i, t := range runs {
		out = append(out, map[string]any{
			"time": i + 1,
			"date": t.Format("2006-01-02 15:04:05"),
		})
	}
	out = append(out, map[string]any{"time": "x", "date": "……"})
	return out, nil
}

func CronDue(expr string, last *int64, now int64) bool {
	e, err := ParseCron(expr)
	if err != nil {
		return false
	}
	if last == nil || *last <= 0 {
		return true
	}
	next := e.Next(time.Unix(*last, 0))
	if next.IsZero() {
		return false
	}
	return !next.After(time.Unix(now, 0))
}

func parseCronField(field string, min, max int) ([]int, error) {
	if field == "" {
		return nil, fmt.Errorf("定时任务运行规则错误")
	}
	out := []int{}
	seen := map[int]bool{}
	add := func(n int) {
		if n < min || n > max || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("定时任务运行规则错误")
		}
		step := 1
		rangePart := part
		if i := strings.Index(part, "/"); i >= 0 {
			rangePart = part[:i]
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
			step = n
		}
		var start, end int
		switch {
		case rangePart == "*":
			start, end = min, max
		case strings.Contains(rangePart, "-"):
			ab := strings.Split(rangePart, "-")
			if len(ab) != 2 {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
			var ok bool
			start, ok = cronFieldValue(ab[0], min, max)
			if !ok {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
			end, ok = cronFieldValue(ab[1], min, max)
			if !ok {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
		default:
			n, ok := cronFieldValue(rangePart, min, max)
			if !ok {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
			start, end = n, n
		}
		if start < min || end > max || start > end {
			return nil, fmt.Errorf("定时任务运行规则错误")
		}
		for n := start; n <= end; n += step {
			add(n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("定时任务运行规则错误")
	}
	return out, nil
}

func expandCronMacros(expr string) string {
	switch strings.ToLower(strings.TrimSpace(expr)) {
	case "@yearly", "@annually":
		return "0 0 1 1 *"
	case "@monthly":
		return "0 0 1 * *"
	case "@weekly":
		return "0 0 * * 0"
	case "@daily", "@midnight":
		return "0 0 * * *"
	case "@hourly":
		return "0 * * * *"
	default:
		return expr
	}
}

var cronMonthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var cronDowNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

func cronFieldValue(s string, min, max int) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	key := strings.ToUpper(s)
	if min == 1 && max == 12 {
		if n, ok := cronMonthNames[key]; ok {
			return n, true
		}
	}
	if min == 0 && max == 7 {
		if n, ok := cronDowNames[key]; ok {
			return n, true
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func containsInt(list []int, v int) bool {
	for _, n := range list {
		if n == v {
			return true
		}
	}
	return false
}
