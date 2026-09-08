package wechat

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

type OAMessage struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	Event        string `xml:"Event"`
	EventKey     string `xml:"EventKey"`
}

func ParseOAXML(raw []byte) (OAMessage, error) {
	var msg OAMessage
	if err := xml.Unmarshal(raw, &msg); err != nil {
		return msg, err
	}
	return msg, nil
}

func TextReplyXML(to, from, content string) string {
	if content == "" {
		return "success"
	}
	esc := func(s string) string {
		return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
	}
	return fmt.Sprintf(`<xml><ToUserName><![CDATA[%s]]></ToUserName><FromUserName><![CDATA[%s]]></FromUserName><CreateTime>%d</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[%s]]></Content></xml>`,
		esc(to), esc(from), time.Now().Unix(), esc(content))
}

const (
	ReplyFollow  = 1
	ReplyKeyword = 2
	ReplyDefault = 3
	MatchFull    = 1
	MatchFuzzy   = 2
)

type ReplyRow struct {
	ID           uint
	Keyword      string
	ReplyType    int
	MatchingType int
	Content      string
	Status       int
	Sort         int
}

func MatchReply(msg OAMessage, rows []ReplyRow) string {
	sorted := append([]ReplyRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Sort != sorted[j].Sort {
			return sorted[i].Sort < sorted[j].Sort
		}
		return sorted[i].ID < sorted[j].ID
	})
	pickByID := func(typ int) string {
		found := false
		bestID := uint(0)
		content := ""
		for _, r := range rows {
			if r.Status != 1 || r.ReplyType != typ || r.Content == "" {
				continue
			}
			if !found || r.ID < bestID {
				found = true
				bestID = r.ID
				content = r.Content
			}
		}
		return content
	}
	if strings.EqualFold(msg.MsgType, "event") && strings.EqualFold(msg.Event, "subscribe") {
		// PHP follow uses value('content') (lowest id), and does not fall through to default.
		return pickByID(ReplyFollow)
	}
	if strings.EqualFold(msg.MsgType, "text") {
		text := msg.Content
		for _, r := range sorted {
			if r.Status != 1 || r.ReplyType != ReplyKeyword || r.Content == "" {
				continue
			}
			ok := r.Keyword == text
			if r.MatchingType == MatchFuzzy {
				ok = strings.Contains(strings.ToLower(text), strings.ToLower(r.Keyword))
			}
			if ok {
				return r.Content
			}
		}
		return pickByID(ReplyDefault)
	}
	return ""
}
