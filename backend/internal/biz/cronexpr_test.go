package biz

import (
	"testing"
	"time"
)

func TestParseCronEveryMinute(t *testing.T) {
	e, err := ParseCron("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 6, 10, 15, 30, 0, time.Local)
	next := e.Next(now)
	want := time.Date(2026, 9, 6, 10, 16, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("next=%s want=%s", next, want)
	}
}

func TestParseCronHourly(t *testing.T) {
	e, err := ParseCron("0 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 6, 10, 15, 0, 0, time.Local)
	next := e.Next(now)
	want := time.Date(2026, 9, 6, 11, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("next=%s want=%s", next, want)
	}
}

func TestParseCronQuestionMark(t *testing.T) {
	dow, err := ParseCron("0 0 * * ?")
	if err != nil {
		t.Fatal(err)
	}
	if !dow.dowStar || !dow.Match(time.Date(2026, 9, 8, 0, 0, 0, 0, time.Local)) {
		t.Fatal("0 0 * * ? should match midnight any weekday")
	}
	dom, err := ParseCron("0 0 ? * 1")
	if err != nil {
		t.Fatal(err)
	}
	if !dom.domStar || !dom.Match(time.Date(2026, 9, 7, 0, 0, 0, 0, time.Local)) { // Monday
		t.Fatal("0 0 ? * 1 should match Monday midnight")
	}
	if ValidCron("0 0 ? * ?") || ValidCron("? * * * *") || ValidCron("0 ? * * *") {
		t.Fatal("invalid ? placements accepted")
	}
}

func TestParseCronInvalid(t *testing.T) {
	if ValidCron("foo") || ValidCron("* * *") || ValidCron("") {
		t.Fatal("expected invalid")
	}
	if ValidCron("5 9 * * * *") {
		t.Fatal("this dragonmantank build has no YearField")
	}
	if ValidCron("0 0 1W,15 * *") || ValidCron("0 0 L,15 * *") {
		t.Fatal("W/L cannot appear in a list")
	}
}

func TestParseCronLastAndNth(t *testing.T) {
	last, err := ParseCron("0 0 L * *")
	if err != nil {
		t.Fatal(err)
	}
	if !last.Match(time.Date(2026, 9, 30, 0, 0, 0, 0, time.Local)) {
		t.Fatal("L should match last day of September")
	}
	if last.Match(time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)) {
		t.Fatal("L should not match the 29th")
	}
	next := last.Next(time.Date(2026, 9, 8, 10, 0, 0, 0, time.Local))
	if !next.Equal(time.Date(2026, 9, 30, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("next last-day=%s", next)
	}

	nth, err := ParseCron("0 9 * * 1#2")
	if err != nil {
		t.Fatal(err)
	}
	// Sept 2026: first Monday=7, second Monday=14
	if !nth.Match(time.Date(2026, 9, 14, 9, 0, 0, 0, time.Local)) {
		t.Fatal("1#2 should match second Monday")
	}
	if nth.Match(time.Date(2026, 9, 7, 9, 0, 0, 0, time.Local)) {
		t.Fatal("1#2 should not match first Monday")
	}

	lastFri, err := ParseCron("0 0 * * 5L")
	if err != nil {
		t.Fatal(err)
	}
	if !lastFri.Match(time.Date(2026, 9, 25, 0, 0, 0, 0, time.Local)) {
		t.Fatal("5L should match last Friday")
	}
	if lastFri.Match(time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local)) {
		t.Fatal("5L should not match the prior Friday")
	}

	wday, err := ParseCron("0 0 15W * *")
	if err != nil {
		t.Fatal(err)
	}
	// 15 Sep 2026 is Tuesday
	if !wday.Match(time.Date(2026, 9, 15, 0, 0, 0, 0, time.Local)) {
		t.Fatal("15W on a Tuesday should stay the 15th")
	}
	sunW, err := ParseCron("0 0 1W * *")
	if err != nil {
		t.Fatal(err)
	}
	// 1 Feb 2026 is Sunday → nearest weekday Monday the 2nd
	if !sunW.Match(time.Date(2026, 2, 2, 0, 0, 0, 0, time.Local)) {
		t.Fatal("1W in Feb 2026 should fire Monday the 2nd")
	}
	if sunW.Match(time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatal("1W should not fire on Sunday the 1st")
	}
	if !ValidCron("0 0 L * *") || !ValidCron("0 9 * * 1#2") || !ValidCron("0 0 * * 7L") {
		t.Fatal("L/# forms should be valid")
	}
}

func TestCronExpressionLists(t *testing.T) {
	lists, err := CronExpressionLists("*/5 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 6 {
		t.Fatalf("len=%d", len(lists))
	}
	if lists[5]["time"] != "x" || lists[5]["date"] != "……" {
		t.Fatalf("tail=%v", lists[5])
	}
	if lists[0]["time"] != 1 {
		t.Fatalf("first time=%v", lists[0]["time"])
	}
}

func TestParseCronMacrosAndNames(t *testing.T) {
	daily, err := ParseCron("@daily")
	if err != nil {
		t.Fatal(err)
	}
	hourly, err := ParseCron("@hourly")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 7, 10, 15, 0, 0, time.Local)
	if next := daily.Next(now); !next.Equal(time.Date(2026, 9, 8, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("daily next=%s", next)
	}
	if next := hourly.Next(now); !next.Equal(time.Date(2026, 9, 7, 11, 0, 0, 0, time.Local)) {
		t.Fatalf("hourly next=%s", next)
	}
	named, err := ParseCron("0 9 * JAN-MAR MON-FRI")
	if err != nil {
		t.Fatal(err)
	}
	if !named.Match(time.Date(2026, 1, 5, 9, 0, 0, 0, time.Local)) { // Monday
		t.Fatal("jan monday 09:00 should match")
	}
	if named.Match(time.Date(2026, 4, 6, 9, 0, 0, 0, time.Local)) { // April Monday
		t.Fatal("april should not match JAN-MAR")
	}
	if named.Match(time.Date(2026, 1, 4, 9, 0, 0, 0, time.Local)) { // Sunday
		t.Fatal("sunday should not match MON-FRI")
	}
	if ValidCron("@reboot") || ValidCron("@yearly extra") {
		t.Fatal("invalid macros")
	}
}

func TestCronDue(t *testing.T) {
	last := time.Date(2026, 9, 6, 10, 0, 0, 0, time.Local).Unix()
	now := time.Date(2026, 9, 6, 10, 1, 0, 0, time.Local).Unix()
	if !CronDue("* * * * *", &last, now) {
		t.Fatal("every minute should be due")
	}
	later := time.Date(2026, 9, 6, 10, 0, 30, 0, time.Local).Unix()
	if CronDue("0 * * * *", &last, later) {
		t.Fatal("hourly should not be due 30s later")
	}
}
