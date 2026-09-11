package router

import "testing"

func TestHashedSPAAsset(t *testing.T) {
	if !hashedSPAAsset("assets/index-AbCdEfGh.js") {
		t.Fatal("assets dir")
	}
	if !hashedSPAAsset("js/app-12345678.js") {
		t.Fatal("hashed js")
	}
	if hashedSPAAsset("index.html") {
		t.Fatal("html is not hashed")
	}
}
