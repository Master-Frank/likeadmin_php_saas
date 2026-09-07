package wechat

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

type encryptedEnvelope struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	Encrypt      string   `xml:"Encrypt"`
	MsgSignature string   `xml:"MsgSignature"`
}

// DecodeOABody unwraps WeChat official-account XML. encryptionType: 1 plaintext, 2 compatible, 3 safe.
func DecodeOABody(raw []byte, token, aesKey, appID string, encryptionType int, msgSig, timestamp, nonce string) (OAMessage, error) {
	if encryptionType <= 1 && !bytes.Contains(raw, []byte("<Encrypt>")) {
		return ParseOAXML(raw)
	}
	var env encryptedEnvelope
	if xml.Unmarshal(raw, &env) != nil || env.Encrypt == "" {
		if encryptionType <= 2 {
			return ParseOAXML(raw)
		}
		return OAMessage{}, fmt.Errorf("加密消息格式错误")
	}
	if msgSig == "" {
		msgSig = env.MsgSignature
	}
	if msgSig != "" && !CheckOAMsgSignature(token, timestamp, nonce, env.Encrypt, msgSig) {
		return OAMessage{}, fmt.Errorf("消息签名错误")
	}
	plain, err := DecryptOA(aesKey, env.Encrypt)
	if err != nil {
		return OAMessage{}, err
	}
	if appID != "" && plain.appID != "" && plain.appID != appID {
		return OAMessage{}, fmt.Errorf("AppID不匹配")
	}
	return ParseOAXML(plain.xml)
}

type oaPlain struct {
	xml   []byte
	appID string
}

func DecryptOA(encodingAESKey, cipherB64 string) (oaPlain, error) {
	key, err := decodeAESKey(encodingAESKey)
	if err != nil {
		return oaPlain{}, err
	}
	raw, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return oaPlain{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return oaPlain{}, err
	}
	if len(raw)%aes.BlockSize != 0 {
		return oaPlain{}, fmt.Errorf("密文长度错误")
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, key[:16]).CryptBlocks(plain, raw)
	plain, err = pkcs7Unpad(plain)
	if err != nil {
		return oaPlain{}, err
	}
	if len(plain) < 20 {
		return oaPlain{}, fmt.Errorf("明文过短")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(msgLen) < 0 || 20+int(msgLen) > len(plain) {
		return oaPlain{}, fmt.Errorf("明文长度错误")
	}
	xmlBody := plain[20 : 20+msgLen]
	appID := string(plain[20+msgLen:])
	return oaPlain{xml: xmlBody, appID: appID}, nil
}

func EncryptOA(encodingAESKey, appID, xmlBody string) (string, error) {
	key, err := decodeAESKey(encodingAESKey)
	if err != nil {
		return "", err
	}
	rand16 := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, rand16); err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(rand16)
	_ = binary.Write(buf, binary.BigEndian, uint32(len(xmlBody)))
	buf.WriteString(xmlBody)
	buf.WriteString(appID)
	padded := pkcs7Pad(buf.Bytes(), aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:16]).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func EncryptedReplyXML(token, aesKey, appID, timestamp, nonce, xmlBody string) (string, error) {
	enc, err := EncryptOA(aesKey, appID, xmlBody)
	if err != nil {
		return "", err
	}
	if timestamp == "" {
		timestamp = strconv.FormatInt(time.Now().Unix(), 10)
	}
	sig := OAMsgSignature(token, timestamp, nonce, enc)
	return fmt.Sprintf(`<xml><Encrypt><![CDATA[%s]]></Encrypt><MsgSignature><![CDATA[%s]]></MsgSignature><TimeStamp>%s</TimeStamp><Nonce><![CDATA[%s]]></Nonce></xml>`,
		enc, sig, timestamp, nonce), nil
}

func OAMsgSignature(token, timestamp, nonce, encrypt string) string {
	arr := []string{token, timestamp, nonce, encrypt}
	sort.Strings(arr)
	sum := sha1.Sum([]byte(strings.Join(arr, "")))
	return hex.EncodeToString(sum[:])
}

func CheckOAMsgSignature(token, timestamp, nonce, encrypt, want string) bool {
	if want == "" {
		return true
	}
	return strings.EqualFold(OAMsgSignature(token, timestamp, nonce, encrypt), want)
}

func decodeAESKey(encodingAESKey string) ([]byte, error) {
	key := strings.TrimSpace(encodingAESKey)
	if key == "" {
		return nil, fmt.Errorf("EncodingAESKey为空")
	}
	if !strings.HasSuffix(key, "=") {
		key += "="
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("EncodingAESKey长度错误")
	}
	return raw, nil
}

func pkcs7Pad(data []byte, block int) []byte {
	pad := block - len(data)%block
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("padding empty")
	}
	pad := int(data[len(data)-1])
	if pad <= 0 || pad > len(data) {
		return nil, fmt.Errorf("padding invalid")
	}
	return data[:len(data)-pad], nil
}
