package bootstrap

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestIsRetryableDBErr(t *testing.T) {
	if isRetryableDBErr(nil) || isRetryableDBErr(gorm.ErrRecordNotFound) {
		t.Fatal("nil / not found")
	}
	if !isRetryableDBErr(errors.New("invalid connection")) {
		t.Fatal("invalid connection")
	}
	if isRetryableDBErr(errors.New("column not found")) {
		t.Fatal("schema error must not trip failover")
	}
}

func TestMarkReplicaUnhealthy(t *testing.T) {
	oldDB, oldRead := DB, ReadDB
	t.Cleanup(func() {
		DB, ReadDB = oldDB, oldRead
		resetReplicaHealth()
	})
	master := &gorm.DB{}
	DB, ReadDB = master, &gorm.DB{}
	replicaHealth.live = true
	MarkReplicaUnhealthy()
	if replicaHealthy() {
		t.Fatal("unhealthy replica must not be used")
	}
	if Read() != master {
		t.Fatal("Read must use master after query failover")
	}
}

func TestUsingReplica(t *testing.T) {
	oldDB, oldRead := DB, ReadDB
	t.Cleanup(func() {
		DB, ReadDB = oldDB, oldRead
	})
	DB, ReadDB = &gorm.DB{}, &gorm.DB{}
	if usingReplica(&gorm.DB{}) {
		t.Fatal("distinct empty sessions are not the replica")
	}
	if usingReplica(nil) {
		t.Fatal("nil")
	}
}
