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

func TestParseCronInvalid(t *testing.T) {
	if ValidCron("foo") || ValidCron("* * *") || ValidCron("") {
		t.Fatal("expected invalid")
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
