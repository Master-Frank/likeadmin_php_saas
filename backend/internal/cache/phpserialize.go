package cache

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

func phpSerialize(v any) (string, error) {
	var b bytes.Buffer
	if err := phpWrite(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func phpWrite(b *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case nil:
		b.WriteString("N;")
	case bool:
		if t {
			b.WriteString("b:1;")
		} else {
			b.WriteString("b:0;")
		}
	case int:
		fmt.Fprintf(b, "i:%d;", t)
	case int8:
		fmt.Fprintf(b, "i:%d;", t)
	case int16:
		fmt.Fprintf(b, "i:%d;", t)
	case int32:
		fmt.Fprintf(b, "i:%d;", t)
	case int64:
		fmt.Fprintf(b, "i:%d;", t)
	case uint:
		fmt.Fprintf(b, "i:%d;", t)
	case uint8:
		fmt.Fprintf(b, "i:%d;", t)
	case uint16:
		fmt.Fprintf(b, "i:%d;", t)
	case uint32:
		fmt.Fprintf(b, "i:%d;", t)
	case uint64:
		fmt.Fprintf(b, "i:%d;", t)
	case float32:
		phpWriteFloat(b, float64(t))
	case float64:
		phpWriteFloat(b, t)
	case string:
		fmt.Fprintf(b, "s:%d:\"%s\";", len(t), t)
	case []byte:
		fmt.Fprintf(b, "s:%d:\"%s\";", len(t), t)
	case []any:
		fmt.Fprintf(b, "a:%d:{", len(t))
		for i, item := range t {
			if err := phpWrite(b, i); err != nil {
				return err
			}
			if err := phpWrite(b, item); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case []string:
		fmt.Fprintf(b, "a:%d:{", len(t))
		for i, item := range t {
			_ = phpWrite(b, i)
			_ = phpWrite(b, item)
		}
		b.WriteByte('}')
	case []int:
		fmt.Fprintf(b, "a:%d:{", len(t))
		for i, item := range t {
			_ = phpWrite(b, i)
			_ = phpWrite(b, item)
		}
		b.WriteByte('}')
	case map[string]any:
		fmt.Fprintf(b, "a:%d:{", len(t))
		for k, item := range t {
			if err := phpWrite(b, k); err != nil {
				return err
			}
			if err := phpWrite(b, item); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return phpWrite(b, fmt.Sprint(t))
	}
	return nil
}

func phpWriteFloat(b *bytes.Buffer, f float64) {
	if f == float64(int64(f)) {
		fmt.Fprintf(b, "i:%d;", int64(f))
		return
	}
	fmt.Fprintf(b, "d:%s;", strconv.FormatFloat(f, 'g', -1, 64))
}

func phpUnserialize(raw string) (any, bool) {
	v, _, ok := phpRead(raw, 0)
	return v, ok
}

func phpRead(s string, i int) (any, int, bool) {
	if i >= len(s) {
		return nil, i, false
	}
	switch s[i] {
	case 'N':
		if i+1 < len(s) && s[i+1] == ';' {
			return nil, i + 2, true
		}
		return nil, i, false
	case 'b':
		if i+3 < len(s) && s[i+1] == ':' && s[i+3] == ';' {
			return s[i+2] == '1', i + 4, true
		}
		return nil, i, false
	case 'i':
		j := strings.IndexByte(s[i+2:], ';')
		if j < 0 {
			return nil, i, false
		}
		n, err := strconv.ParseInt(s[i+2:i+2+j], 10, 64)
		if err != nil {
			return nil, i, false
		}
		return n, i + 3 + j, true
	case 'd':
		j := strings.IndexByte(s[i+2:], ';')
		if j < 0 {
			return nil, i, false
		}
		f, err := strconv.ParseFloat(s[i+2:i+2+j], 64)
		if err != nil {
			return nil, i, false
		}
		return f, i + 3 + j, true
	case 's':
		colon := strings.IndexByte(s[i+2:], ':')
		if colon < 0 {
			return nil, i, false
		}
		n, err := strconv.Atoi(s[i+2 : i+2+colon])
		if err != nil {
			return nil, i, false
		}
		start := i + 3 + colon
		if start >= len(s) || s[start] != '"' {
			return nil, i, false
		}
		start++
		end := start + n
		if end+1 >= len(s) || s[end] != '"' || s[end+1] != ';' {
			return nil, i, false
		}
		return s[start:end], end + 2, true
	case 'a':
		colon := strings.IndexByte(s[i+2:], ':')
		if colon < 0 {
			return nil, i, false
		}
		n, err := strconv.Atoi(s[i+2 : i+2+colon])
		if err != nil {
			return nil, i, false
		}
		pos := i + 3 + colon
		if pos >= len(s) || s[pos] != '{' {
			return nil, i, false
		}
		pos++
		obj := map[string]any{}
		arr := make([]any, 0, n)
		isList := true
		for k := 0; k < n; k++ {
			key, next, ok := phpRead(s, pos)
			if !ok {
				return nil, i, false
			}
			val, next2, ok := phpRead(s, next)
			if !ok {
				return nil, i, false
			}
			pos = next2
			switch t := key.(type) {
			case int64:
				if int(t) != k {
					isList = false
				}
				obj[strconv.FormatInt(t, 10)] = val
				if isList {
					arr = append(arr, val)
				}
			case string:
				isList = false
				obj[t] = val
			default:
				isList = false
				obj[fmt.Sprint(t)] = val
			}
		}
		if pos >= len(s) || s[pos] != '}' {
			return nil, i, false
		}
		if isList && len(arr) == n {
			return arr, pos + 1, true
		}
		return obj, pos + 1, true
	default:
		return nil, i, false
	}
}
