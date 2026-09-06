package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func MD5(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// CreatePassword mirrors PHP create_password: md5(salt + md5(plaintext + salt))
func CreatePassword(plaintext, salt string) string {
	return MD5(salt + MD5(plaintext+salt))
}

// CreateToken mirrors PHP create_token.
func CreateToken(extra string, salt string) string {
	if salt == "" {
		salt = "likeadmin"
	}
	encryptSalt := MD5(salt + fmt.Sprintf("%d", time.Now().UnixNano()))
	return MD5(salt + extra + strconv.FormatInt(time.Now().Unix(), 10) + encryptSalt)
}

func ParseDateTime(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05", "2006-01-02", time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.Unix()
		}
	}
	if n := ParseInt(s); n > 0 {
		return int64(n)
	}
	return 0
}

func FormatDateTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).In(time.Local).Format("2006-01-02 15:04:05")
}

func FormatDateTimeMinute(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).In(time.Local).Format("2006-01-02 15:04")
}

func FormatDateTimePtr(ts *int64) string {
	if ts == nil || *ts <= 0 {
		return ""
	}
	return FormatDateTime(*ts)
}

func FormatDateTimeOrNil(ts *int64) any {
	if ts == nil || *ts <= 0 {
		return nil
	}
	return FormatDateTime(*ts)
}

func NowUnix() int64 {
	return time.Now().Unix()
}
