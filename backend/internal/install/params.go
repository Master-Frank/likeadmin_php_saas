package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
)

// CheckParams mirrors PHP install.php step-4 field checks.
func CheckParams(p map[string]any) string {
	if prefix := pick(p, "prefix"); prefix == "" || prefix == "0" {
		return "数据表前缀不能为空"
	}
	if pick(p, "admin_user") == "" {
		return "请填写管理员用户名"
	}
	if strings.TrimSpace(pick(p, "admin_password")) == "" {
		return "管理员密码不能为空"
	}
	if pick(p, "admin_password") != pick(p, "admin_confirm_password") {
		return "两次密码不一致"
	}
	return ""
}

func pick(p map[string]any, key string) string {
	if p == nil {
		return ""
	}
	return util.ToString(p[key])
}

func isOn(p map[string]any, key string) bool {
	v := strings.ToLower(strings.TrimSpace(pick(p, key)))
	return v == "on" || v == "1" || v == "true" || v == "yes"
}

// AccountSalt mirrors PHP initAccount: substr(md5(time . admin_user), 0, 4).
func AccountSalt(ts int64, adminUser string) string {
	sum := util.MD5(fmt.Sprintf("%d%s", ts, adminUser))
	if len(sum) < 4 {
		return sum
	}
	return sum[:4]
}

func insertAdmin(db *gorm.DB, prefix, adminUser, adminPass string, ts int64) (salt string, err error) {
	salt = AccountSalt(ts, adminUser)
	pwd := util.CreatePassword(adminPass, salt)
	adminTable := "`" + prefix + "admin`"
	deptTable := "`" + prefix + "admin_dept`"
	if err = db.Exec("INSERT INTO "+adminTable+
		"(`id`, `root`, `name`, `avatar`, `account`, `password`, `login_time`, `login_ip`, `multipoint_login`, `disable`, `create_time`, `update_time`, `delete_time`) VALUES (1, 1, ?, '', ?, ?, ?, '', 1, 0, ?, ?, NULL)",
		adminUser, adminUser, pwd, ts, ts, ts).Error; err != nil {
		return "", err
	}
	if err = db.Exec("INSERT INTO " + deptTable + " (`admin_id`, `dept_id`) VALUES (1, 1)").Error; err != nil {
		return "", err
	}
	return salt, nil
}

func tableExists(db *gorm.DB, dbName, prefix string) bool {
	if db == nil || !identOK(dbName) || !identOK(prefix) {
		return false
	}
	// MariaDB rejects placeholders on SHOW TABLES LIKE; PHP interpolates the name.
	like := prefix + "config"
	q := fmt.Sprintf("SHOW TABLES FROM `%s` LIKE '%s'", dbName, like)
	var name string
	_ = db.Raw(q).Scan(&name)
	return name != ""
}

func identOK(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return true
}

func importDemo(db *gorm.DB, publicDir, prefix, dbName string) error {
	cands := []string{
		filepath.Join(publicDir, "install", "db", "ys.sql"),
		filepath.Join(publicDir, "..", "public", "install", "db", "ys.sql"),
	}
	var path string
	for _, p := range cands {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			path = p
			break
		}
	}
	if path == "" {
		return fmt.Errorf("导入测试数据错误")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("导入测试数据错误")
	}
	if _, err := ImportSQL(db, string(raw), prefix, dbName); err != nil {
		return fmt.Errorf("导入测试数据错误")
	}
	from := filepath.Join(filepath.Dir(path), "..", "uploads")
	to := filepath.Join(publicDir, "uploads")
	_ = copyDir(from, to)
	return nil
}

func copyDir(src, dest string) error {
	st, err := os.Stat(src)
	if err != nil || !st.IsDir() {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o777)
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
			return err
		}
		if err := os.WriteFile(target, raw, info.Mode()); err != nil {
			return err
		}
		// PHP installModel::cpFiles chmod 0777 when dest contains access_token.txt.
		if strings.Contains(target, "access_token.txt") {
			_ = os.Chmod(target, 0o777)
		}
		return nil
	})
}

func nowUnix() int64 { return time.Now().Unix() }
