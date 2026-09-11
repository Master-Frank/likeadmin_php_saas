package bootstrap

import (
	"testing"
	"time"
)

func TestReplicaMaxLagDefault(t *testing.T) {
	t.Setenv("LIKEADMIN_REPLICA_MAX_LAG", "")
	if replicaMaxLag() != 30*time.Second {
		t.Fatalf("%s", replicaMaxLag())
	}
	t.Setenv("LIKEADMIN_REPLICA_MAX_LAG", "5")
	if replicaMaxLag() != 5*time.Second {
		t.Fatalf("%s", replicaMaxLag())
	}
}

func TestAsSeconds(t *testing.T) {
	d, ok := asSeconds([]byte("12"))
	if !ok || d != 12*time.Second {
		t.Fatalf("%v %v", d, ok)
	}
	if _, ok := asSeconds(nil); ok {
		t.Fatal("nil")
	}
}
