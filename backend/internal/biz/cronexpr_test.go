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
