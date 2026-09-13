package util

import (
	"fmt"
	"math/rand"
	"time"
)

// GenerateSN mirrors PHP generate_sn: prefix + YmdHis + N random digits.
func GenerateSN(exists func(string) bool, prefix string, suffixLen int) string {
	if suffixLen <= 0 {
		suffixLen = 4
	}
	for i := 0; i < 32; i++ {
		sn := prefix + time.Now().Format("20060102150405") + randDigits(suffixLen)
		if exists == nil || !exists(sn) {
			return sn
		}
	}
	return prefix + time.Now().Format("20060102150405") + fmt.Sprintf("%0*d", suffixLen, time.Now().UnixNano()%int64(pow10(suffixLen)))
}

// CreateUserSN mirrors PHP User::createUserSn: 8 digits each in 1-9.
func CreateUserSN(exists func(int) bool) int {
	for i := 0; i < 32; i++ {
		n := 0
		for j := 0; j < 8; j++ {
			n = n*10 + rand.Intn(9) + 1
		}
		if exists == nil || !exists(n) {
			return n
		}
	}
	return int(time.Now().Unix()%90000000) + 10000000
}

const tenantSNChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// CreateTenantSN mirrors PHP Tenant::createUserSn: prefix + 8 chars from [a-z0-9].
// PHP uses mt_rand(0, 35). Do not derive the index from a unix-seeded LCG:
// Go's remainder keeps the sign, so int64 overflow panics on chars[negative].
func CreateTenantSN(exists func(string) bool) string {
	return createTenantSN(exists, "", 8)
}

func createTenantSN(exists func(string) bool, prefix string, length int) string {
	if length <= 0 {
		length = 8
	}
	for i := 0; i < 64; i++ {
		sn := prefix + randCharset(tenantSNChars, length)
		if exists == nil || !exists(sn) {
			return sn
		}
	}
	return prefix + randCharset(tenantSNChars, length)
}

func randCharset(chars string, n int) string {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func ZeroUnixPtr() *int64 {
	z := int64(0)
	return &z
}

func randDigits(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = byte('0' + rand.Intn(10))
	}
	return string(b)
}

func pow10(n int) int {
	p := 1
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}
