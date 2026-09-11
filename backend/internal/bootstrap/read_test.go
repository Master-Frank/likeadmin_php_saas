package bootstrap

import (
	"testing"

	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

func TestReadFallsBackToMaster(t *testing.T) {
	oldDB, oldRead := DB, ReadDB
	t.Cleanup(func() {
		DB, ReadDB = oldDB, oldRead
	})
	master := &gorm.DB{}
	DB, ReadDB = master, nil
	if Read() != master {
		t.Fatal("empty replica must use master")
	}
	rep := &gorm.DB{}
	ReadDB = rep
	if Read() != rep {
		t.Fatal("configured replica must be used for reads")
	}
}

func TestBindReadDBEmptyReplica(t *testing.T) {
	oldDB, oldRead, oldCfg := DB, ReadDB, config.C.Database
	t.Cleanup(func() {
		DB, ReadDB = oldDB, oldRead
		config.C.Database = oldCfg
	})
	config.C.Database.Replicas = nil
	master := &gorm.DB{}
	bindReadDB(master)
	if ReadDB != master {
		t.Fatal("no replica config must keep ReadDB on master")
	}
}

func TestFillReplicaInheritsMaster(t *testing.T) {
	got := fillReplica(config.DatabaseConfig{Hostname: "10.0.0.2"}, config.DatabaseConfig{
		Hostname: "10.0.0.1", Hostport: 3306, Database: "app", Username: "u", Password: "p",
		Charset: "utf8mb4", Prefix: "la_", MaxOpenConns: 50, MaxIdleConns: 10,
		ConnMaxLifetime: 300, ConnMaxIdleTime: 60,
	})
	if got.Hostname != "10.0.0.2" || got.Database != "app" || got.Username != "u" || got.Hostport != 3306 {
		t.Fatalf("%+v", got)
	}
	if got.MaxOpenConns != 50 || got.Prefix != "la_" {
		t.Fatalf("pool/prefix %+v", got)
	}
}
