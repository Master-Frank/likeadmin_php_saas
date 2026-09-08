package install

import (
	"os"

	"likeadmin/backend/internal/config"

	"github.com/spf13/viper"
)

// WriteGoConfig persists install DB/login salt into the running Go config
// (memory + config.yaml) so platform login works without a hand edit.
func WriteGoConfig(path, host, dbName, user, pass string, port int, prefix, httpHost, uniqueID string) error {
	if prefix == "" {
		prefix = "la_"
	}
	if port == 0 {
		port = 3306
	}
	if uniqueID == "" {
		uniqueID = "likeadmin"
	}
	config.C.Database.Hostname = host
	config.C.Database.Hostport = port
	config.C.Database.Database = dbName
	config.C.Database.Username = user
	config.C.Database.Password = pass
	config.C.Database.Charset = "utf8mb4"
	config.C.Database.Prefix = prefix
	config.C.Project.UniqueIdentification = uniqueID
	if httpHost != "" {
		config.C.Project.HTTPHost = httpHost
	}
	if path == "" {
		path = config.Path
	}
	if path == "" {
		return nil
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			return err
		}
		v.SetConfigType("yaml")
	}
	v.Set("database.hostname", host)
	v.Set("database.hostport", port)
	v.Set("database.database", dbName)
	v.Set("database.username", user)
	v.Set("database.password", pass)
	v.Set("database.charset", "utf8mb4")
	v.Set("database.prefix", prefix)
	v.Set("project.unique_identification", uniqueID)
	if httpHost != "" {
		v.Set("project.http_host", httpHost)
	}
	if err := v.WriteConfig(); err != nil {
		return v.WriteConfigAs(path)
	}
	return nil
}
