package upgrade

import "likeadmin/backend/internal/util"

// AuthFailMsg mirrors PHP UpgradeLogic verify-fail copy.
func AuthFailMsg(result map[string]any) string {
	if msg := util.ToString(result["msg"]); msg != "" {
		return msg
	}
	return "请先联系客服获取授权"
}

// ApplyAuthorized is the PHP UpgradeLogic::upgrade success path after
// CheckAbleUpgrade / open_basedir: verify → apply zip → persist version → log.
func ApplyAuthorized(domain string, versionID any) error {
	result := Verify(domain, versionID, "package_link")
	if !HasPermission(result) {
		msg := AuthFailMsg(result)
		AddLog(domain, versionID, 1, false, msg)
		return errStatus(msg)
	}
	if err := ApplyPackage(util.ToString(result["link"]), ""); err != nil {
		AddLog(domain, versionID, 1, false, err.Error())
		return err
	}
	if ver := VersionByID(versionID); ver != nil {
		_ = WriteLocalVersion(util.ToString(ver["version_no"]))
	}
	AddLog(domain, versionID, 1, true, "")
	return nil
}
