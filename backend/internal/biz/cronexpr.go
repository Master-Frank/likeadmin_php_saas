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
			var err error
			start, err = strconv.Atoi(ab[0])
			if err != nil {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
			end, err = strconv.Atoi(ab[1])
			if err != nil {
				return nil, fmt.Errorf("定时任务运行规则错误")
			}
		default:
			n, err := strconv.Atoi(rangePart)
			if err != nil {
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

func containsInt(list []int, v int) bool {
	for _, n := range list {
		if n == v {
			return true
		}
	}
	return false
}
