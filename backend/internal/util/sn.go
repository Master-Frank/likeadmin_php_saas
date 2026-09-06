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
