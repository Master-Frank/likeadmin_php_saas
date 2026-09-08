package gencrud

import (
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
)

// PHPActionNotes matches generated PHP controller @notes: 获取{NOTES}列表 / 添加{NOTES} / …
func PHPActionNotes(ctrl, action string) string {
	if bootstrap.DB == nil {
		return ""
	}
	ctrl = strings.ToLower(strings.TrimSpace(ctrl))
	action = strings.ToLower(strings.TrimSpace(action))
	if ctrl == "" {
		return ""
	}
	var tables []model.GenerateTable
	if bootstrap.DB.Find(&tables).Error != nil {
		return ""
	}
	notes := ""
	for _, t := range tables {
		if RouteKey(t) != ctrl {
			continue
		}
		notes = strings.TrimSpace(t.ClassComment)
		if notes == "" {
			notes = strings.TrimSpace(t.TableComment)
		}
		break
	}
	if notes == "" {
		return ""
	}
	return phpNotesLabel(notes, action)
}

func phpNotesLabel(notes, action string) string {
	switch action {
	case actionLists:
		return "获取" + notes + "列表"
	case actionAdd:
		return "添加" + notes
	case actionEdit:
		return "编辑" + notes
	case actionDelete:
		return "删除" + notes
	case actionDetail:
		return "获取" + notes + "详情"
	default:
		return ""
	}
}
