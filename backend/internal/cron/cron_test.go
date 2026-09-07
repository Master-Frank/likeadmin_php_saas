package cron

import (
	"testing"

	"likeadmin/backend/internal/model"
)

func TestNormalizeCommand(t *testing.T) {
	if normalizeCommand(`app\common\command\QueryRefund`) != "query_refund" {
		t.Fatal(normalizeCommand(`app\common\command\QueryRefund`))
	}
	if normalizeCommand("crontab") != "crontab" {
		t.Fatal(normalizeCommand("crontab"))
	}
	if normalizeCommand("clear_session") != "session" {
		t.Fatal(normalizeCommand("clear_session"))
	}
	if normalizeCommand("clear") != "clear" {
		t.Fatal(normalizeCommand("clear"))
	}
}

func TestRunCommandUnknown(t *testing.T) {
	got := runCommand(model.Crontab{Command: "not_a_real_command"})
	if got != "未定义的定时任务命令: not_a_real_command" {
		t.Fatalf("got %q", got)
	}
}

func TestRunNamed(t *testing.T) {
	if RunNamed("not_a_real_command") != "未定义的定时任务命令: not_a_real_command" {
		t.Fatal(RunNamed("not_a_real_command"))
	}
	if got := RunNamed(`app\common\command\QueryRefund`); len(got) >= 3 && got[:3] == "未定" {
		t.Fatalf("native query_refund should not fall through, got %q", got)
	}
	if got := RunNamed("crontab"); got != "" {
		t.Fatalf("think crontab is RunOnce, got %q", got)
	}
	if got := runCommand(model.Crontab{Command: "crontab"}); got != "未定义的定时任务命令: crontab" {
		t.Fatalf("db row crontab must not recurse: %q", got)
	}
}
