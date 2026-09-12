package install

import (
	"os"

	"likeadmin/backend/internal/config"

	"github.com/spf13/viper"
)

// GoWrite is the install-time Go yaml payload. Empty replica/redis fields
// keep today's single-node defaults.
type GoWrite struct {
	Path, Host, DBName, User, Pass, Prefix, HTTPHost, UniqueID string
	Port                                                       int
	MultiInstance, ExportAsync, RequireRedis                   bool
	CDNDomain                                                  string
	RedisHost, RedisPassword                                   string
	RedisPort, RedisDB                                         int
	ReplicaHost, ReplicaUser, ReplicaPass, ReplicaName         string
	ReplicaPort                                                int
}

// WriteGoConfig persists install DB/login salt into the running Go config
// (memory + config.yaml) so platform login works without a hand edit.
func WriteGoConfig(path, host, dbName, user, pass string, port int, prefix, httpHost, uniqueID string) error {
	return WriteGoConfigOpts(GoWrite{
		Path: path, Host: host, DBName: dbName, User: user, Pass: pass,
		Port: port, Prefix: prefix, HTTPHost: httpHost, UniqueID: uniqueID,
	})
}

func WriteGoConfigOpts(opt GoWrite) error {
	if opt.Prefix == "" {
		opt.Prefix = "la_"
	}
	if opt.Port == 0 {
		opt.Port = 3306
	}
	if opt.UniqueID == "" {
		opt.UniqueID = "likeadmin"
	}
	config.C.Database.Hostname = opt.Host
	config.C.Database.Hostport = opt.Port
	config.C.Database.Database = opt.DBName
	config.C.Database.Username = opt.User
	config.C.Database.Password = opt.Pass
	config.C.Database.Charset = "utf8mb4"
	config.C.Database.Prefix = opt.Prefix
	config.C.Project.UniqueIdentification = opt.UniqueID
	config.C.App.MultiInstance = opt.MultiInstance
	config.C.App.RequireRedis = opt.RequireRedis
	config.C.App.CDNDomain = opt.CDNDomain
	config.C.Project.ExportAsync = opt.ExportAsync
	if opt.HTTPHost != "" {
		config.C.Project.HTTPHost = opt.HTTPHost
	}
	if opt.RedisHost != "" {
		config.C.Redis.Host = opt.RedisHost
		if opt.RedisPort > 0 {
			config.C.Redis.Port = opt.RedisPort
		}
		config.C.Redis.Password = opt.RedisPassword
		config.C.Redis.DB = opt.RedisDB
	}
	if opt.ReplicaHost != "" {
		rep := config.DatabaseConfig{
			Hostname: opt.ReplicaHost, Hostport: opt.ReplicaPort,
			Database: opt.ReplicaName, Username: opt.ReplicaUser,
			Password: opt.ReplicaPass, Charset: "utf8mb4", Prefix: opt.Prefix,
		}
		if rep.Hostport == 0 {
			rep.Hostport = opt.Port
		}
		if rep.Database == "" {
			rep.Database = opt.DBName
		}
		if rep.Username == "" {
			rep.Username = opt.User
		}
		if rep.Password == "" {
			rep.Password = opt.Pass
		}
		config.C.Database.Replicas = []config.DatabaseConfig{rep}
	} else {
		config.C.Database.Replicas = nil
	}
	path := opt.Path
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
	v.Set("database.hostname", opt.Host)
	v.Set("database.hostport", opt.Port)
	v.Set("database.database", opt.DBName)
	v.Set("database.username", opt.User)
	v.Set("database.password", opt.Pass)
	v.Set("database.charset", "utf8mb4")
	v.Set("database.prefix", opt.Prefix)
	v.Set("project.unique_identification", opt.UniqueID)
	v.Set("app.multi_instance", opt.MultiInstance)
	v.Set("app.require_redis", opt.RequireRedis)
	v.Set("app.cdn_domain", opt.CDNDomain)
	v.Set("project.export_async", opt.ExportAsync)
	if opt.HTTPHost != "" {
		v.Set("project.http_host", opt.HTTPHost)
	}
	if opt.RedisHost != "" {
		v.Set("redis.host", opt.RedisHost)
		if opt.RedisPort > 0 {
			v.Set("redis.port", opt.RedisPort)
		}
		v.Set("redis.password", opt.RedisPassword)
		v.Set("redis.db", opt.RedisDB)
	}
	if opt.ReplicaHost != "" {
		port := opt.ReplicaPort
		if port == 0 {
			port = opt.Port
		}
		name := opt.ReplicaName
		if name == "" {
			name = opt.DBName
		}
		user := opt.ReplicaUser
		if user == "" {
			user = opt.User
		}
		pass := opt.ReplicaPass
		if pass == "" {
			pass = opt.Pass
		}
		v.Set("database.replicas", []map[string]any{{
			"hostname": opt.ReplicaHost,
			"hostport": port,
			"database": name,
			"username": user,
			"password": pass,
			"charset":  "utf8mb4",
			"prefix":   opt.Prefix,
		}})
	}
	if err := v.WriteConfig(); err != nil {
		return v.WriteConfigAs(path)
	}
	return nil
}
