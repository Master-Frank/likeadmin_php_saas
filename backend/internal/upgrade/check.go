package upgrade

import "likeadmin/backend/internal/util"

// CheckAbleUpgrade mirrors UpgradeValidate::checkIsAbleUpgrade.
func CheckAbleUpgrade(id any) string {
	return checkAbleUpgrade(LocalVersion(), VersionByID(id), GetRemoteVersion(0, 0))
}

func checkAbleUpgrade(local string, target map[string]any, payload map[string]any) string {
	if len(target) == 0 {
		return "未获取到对应版本信息"
	}
	targetNo := util.ToString(target["version_no"])
	if local > targetNo {
		return "当前系统无法升级到该版本，请逐个版本升级。"
	}
	remote, _ := payload["lists"].([]any)
	if len(remote) == 0 {
		return "获取更新数据失败"
	}
	for k, item := range remote {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		if util.ToString(m["version_no"]) != local {
			continue
		}
		if k == 0 {
			return "已为最新版本"
		}
		prev, _ := remote[k-1].(map[string]any)
		if prev == nil || util.ToString(prev["version_no"]) != targetNo {
			return "当前系统无法升级到该版本，请逐个版本升级。"
		}
		return ""
	}
	return ""
}

// CheckVersionData mirrors downloadPkgValidate::checkVersionData.
func CheckVersionData(id any) string {
	if len(VersionByID(id)) == 0 {
		return "未获取到对应版本信息"
	}
	return ""
}
