package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Pgsql    DatabaseConfig `mapstructure:"pgsql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Project  ProjectConfig  `mapstructure:"project"`
}

type AppConfig struct {
	Debug       bool   `mapstructure:"debug"`
	Timezone    string `mapstructure:"timezone"`
	PublicDir   string `mapstructure:"public_dir"`
	InstallLock string `mapstructure:"install_lock"`
	Listen      string `mapstructure:"listen"`
}

type DatabaseConfig struct {
	Hostname string `mapstructure:"hostname"`
	Hostport int    `mapstructure:"hostport"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Charset  string `mapstructure:"charset"`
	Prefix   string `mapstructure:"prefix"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

type TokenConfig struct {
	ExpireDuration   int `mapstructure:"expire_duration"`
	BeExpireDuration int `mapstructure:"be_expire_duration"`
}

type ListsConfig struct {
	PageSizeMax int `mapstructure:"page_size_max"`
	PageSize    int `mapstructure:"page_size"`
}

type ProjectConfig struct {
	UniqueIdentification string            `mapstructure:"unique_identification"`
	DefaultPassword      string            `mapstructure:"default_password"`
	DemoEnv              bool              `mapstructure:"demo_env"`
	HTTPHost             string            `mapstructure:"http_host"`
	Version              string            `mapstructure:"version"`
	ProjectName          string            `mapstructure:"project_name"`
	AdminToken           TokenConfig       `mapstructure:"admin_token"`
	TenantToken          TokenConfig       `mapstructure:"tenant_token"`
	UserToken            TokenConfig       `mapstructure:"user_token"`
	Lists                ListsConfig       `mapstructure:"lists"`
	DefaultImage         map[string]string `mapstructure:"default_image"`
	FileImage            []string          `mapstructure:"file_image"`
	FileVideo            []string          `mapstructure:"file_video"`
	FileFile             []string          `mapstructure:"file_file"`
	Platform             map[string]string `mapstructure:"platform"`
	Tenant               map[string]string `mapstructure:"tenant"`
	Website              map[string]string `mapstructure:"website"`
	Login                map[string]any    `mapstructure:"login"`
	Decorate             map[string]any    `mapstructure:"decorate"`
}

var (
	C    Config
	Path string
)

func Load(path string) error {
	Path = path
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(&C); err != nil {
		return err
	}
	abs, err := filepath.Abs(filepath.Dir(path))
	if err == nil {
		if !filepath.IsAbs(C.App.PublicDir) {
			C.App.PublicDir = filepath.Clean(filepath.Join(abs, C.App.PublicDir))
		}
		if !filepath.IsAbs(C.App.InstallLock) {
			C.App.InstallLock = filepath.Clean(filepath.Join(abs, C.App.InstallLock))
		}
	}
	if env := os.Getenv("LIKEADMIN_LISTEN"); env != "" {
		C.App.Listen = env
	}
	return nil
}

func Prefix() string {
	if C.Database.Prefix == "" {
		return "la_"
	}
	return C.Database.Prefix
}
