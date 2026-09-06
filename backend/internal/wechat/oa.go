package wechat

import (
	"encoding/xml"
	"fmt"
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
	Keyword      string
	ReplyType    int
	MatchingType int
	Content      string
	Status       int
	Sort         int
}

func MatchReply(msg OAMessage, rows []ReplyRow) string {
	if strings.EqualFold(msg.MsgType, "event") && strings.EqualFold(msg.Event, "subscribe") {
		for _, r := range rows {
			if r.Status == 1 && r.ReplyType == ReplyFollow && r.Content != "" {
				return r.Content
			}
		}
	}
	if strings.EqualFold(msg.MsgType, "text") {
		text := msg.Content
		best := ""
		bestSort := int(^uint(0) >> 1)
		for _, r := range rows {
			if r.Status != 1 || r.ReplyType != ReplyKeyword || r.Content == "" {
				continue
			}
			ok := false
			if r.MatchingType == MatchFuzzy {
				ok = strings.Contains(strings.ToLower(text), strings.ToLower(r.Keyword))
			} else {
				ok = r.Keyword == text
			}
			if ok && r.Sort < bestSort {
				best = r.Content
				bestSort = r.Sort
			}
		}
		if best != "" {
			return best
		}
		for _, r := range rows {
			if r.Status == 1 && r.ReplyType == ReplyDefault && r.Content != "" {
				return r.Content
			}
		}
	}
	return ""
}
