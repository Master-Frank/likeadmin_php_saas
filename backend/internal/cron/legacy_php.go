package cron

import (
	"os"
	"strings"
)

const legacyPHPDisabled = "PHP 脚手架默认关闭；如需兼容输出请设置 LIKEADMIN_ENABLE_PHP_SCAFFOLD=1"

func legacyPHPScaffoldEnabled() bool {
	return strings.TrimSpace(os.Getenv("LIKEADMIN_ENABLE_PHP_SCAFFOLD")) == "1"
}
