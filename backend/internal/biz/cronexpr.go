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
	domLast                    bool
	domW                       int // nearest weekday to this DOM; 0 unused
	dowLast                    int // last weekday 0-6; -1 unused
	dowNth                     int // 1-5; 0 unused
	dowNthWeekday              int // 0-6
}

func ParseCron(expr string) (*CronExpr, error) {
	expr = expandCronMacros(expr)
	parts := strings.Fields(strings.TrimSpace(expr))
	// This dragonmantank build has no YearField; a 6th part fails setPart(5).
	if len(parts) != 5 {
		return nil, fmt.Errorf("定时任务运行规则错误")
	}
	// dragonmantank CronExpression: ? is only valid on DOM or DOW, and not both.
	if parts[0] == "?" || parts[1] == "?" || parts[3] == "?" {
		return nil, fmt.Errorf("定时任务运行规则错误")
	}
	if parts[2] == "?" && parts[4] == "?" {
		return nil, fmt.Errorf("定时任务运行规则错误")
	}
	domStar := parts[2] == "*" || parts[2] == "?"
	dowStar := parts[4] == "*" || parts[4] == "?"
	if parts[2] == "?" {
		parts[2] = "*"
	}
	if parts[4] == "?" {
		parts[4] = "*"
	}
	min, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return nil, err
	}
	hour, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return nil, err
	}
	month, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return nil, err
	}
	e := &CronExpr{
		min: min, hour: hour, month: month,
		domStar: domStar, dowStar: dowStar, dowLast: -1,
	}
	if err := parseDOM(parts[2], e); err != nil {
		return nil, err
	}
	if err := parseDOW(parts[4], e); err != nil {
		return nil, err
	}
	return e, nil
}

func ValidCron(expr string) bool {
	_, err := ParseCron(expr)
	return err == nil
}

func (e *CronExpr) Match(t time.Time) bool {
	if !containsInt(e.min, t.Minute()) || !containsInt(e.hour, t.Hour()) || !containsInt(e.month, int(t.Month())) {
		return false
	}
	domOK := e.matchDOM(t)
	dowOK := e.matchDOW(t)
	if e.domStar || e.dowStar {
		return (e.domStar || domOK) && (e.dowStar || dowOK)
	}
	return domOK || dowOK
}

func (e *CronExpr) matchDOM(t time.Time) bool {
	if e.domLast {
		return t.Day() == lastDayOfMonth(t)
	}
	if e.domW > 0 {
		return t.Day() == nearestWeekday(t.Year(), t.Month(), e.domW)
	}
	return containsInt(e.dom, t.Day())
}

func (e *CronExpr) matchDOW(t time.Time) bool {
	wd := int(t.Weekday())
	if e.dowLast >= 0 {
		return wd == e.dowLast && lastDayOfMonth(t)-t.Day() < 7
	}
	if e.dowNth > 0 {
		return wd == e.dowNthWeekday && t.Day() == nthWeekdayDate(t.Year(), t.Month(), time.Weekday(e.dowNthWeekday), e.dowNth)
	}
	return containsInt(e.dow, wd)
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

func parseDOM(field string, e *CronExpr) error {
	if field == "L" {
		e.domLast = true
		return nil
	}
	if strings.Contains(field, ",") && (strings.Contains(field, "W") || strings.Contains(field, "L")) {
		return fmt.Errorf("定时任务运行规则错误")
	}
	if strings.HasSuffix(field, "W") {
		n, ok := cronFieldValue(strings.TrimSuffix(field, "W"), 1, 31)
		if !ok || n < 1 || n > 31 {
			return fmt.Errorf("定时任务运行规则错误")
		}
		e.domW = n
		return nil
	}
	vals, err := parseCronField(field, 1, 31)
	if err != nil {
		return err
	}
	e.dom = vals
	return nil
}

func parseDOW(field string, e *CronExpr) error {
	if i := strings.Index(field, "#"); i >= 0 {
		wd, ok := cronFieldValue(field[:i], 0, 7)
		if !ok {
			return fmt.Errorf("定时任务运行规则错误")
		}
		nth, err := strconv.Atoi(field[i+1:])
		if err != nil || nth < 1 || nth > 5 {
			return fmt.Errorf("定时任务运行规则错误")
		}
		if wd == 7 {
			wd = 0
		}
		e.dowNth = nth
		e.dowNthWeekday = wd
		return nil
	}
	if strings.HasSuffix(field, "L") && field != "L" {
		wd, ok := cronFieldValue(strings.TrimSuffix(field, "L"), 0, 7)
		if !ok {
			return fmt.Errorf("定时任务运行规则错误")
		}
		if wd == 7 {
			wd = 0
		}
		e.dowLast = wd
		return nil
	}
	vals, err := parseCronField(field, 0, 7)
	if err != nil {
		return err
	}
	norm := make([]int, 0, len(vals))
	seen := map[int]bool{}
	for _, d := range vals {
		if d == 7 {
			d = 0
		}
		if !seen[d] {
			seen[d] = true
			norm = append(norm, d)
		}
	}
	e.dow = norm
	return nil
}

func lastDayOfMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

func nearestWeekday(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	if day < 1 || day > last {
		return 0
	}
	target := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	if target.Weekday() != time.Saturday && target.Weekday() != time.Sunday {
		return day
	}
	for _, i := range []int{-1, 1, -2, 2} {
		adj := day + i
		if adj < 1 || adj > last {
			continue
		}
		t := time.Date(year, month, adj, 0, 0, 0, 0, time.Local)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			return adj
		}
	}
	return 0
}

func nthWeekdayDate(year int, month time.Month, weekday time.Weekday, nth int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	count := 0
	for d := 1; d <= last; d++ {
		t := time.Date(year, month, d, 0, 0, 0, 0, time.Local)
		if t.Weekday() == weekday {
			count++
			if count == nth {
				return d
			}
		}
	}
	return 0
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
