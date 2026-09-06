package util

import (
	"encoding/json"
	"strings"
)

func LinearToTree(data []map[string]any, subKey, idName, parentIDName string, parentID any) []map[string]any {
	if subKey == "" {
		subKey = "sub"
	}
	if idName == "" {
		idName = "id"
	}
	if parentIDName == "" {
		parentIDName = "pid"
	}
	tree := make([]map[string]any, 0)
	for _, row := range data {
		if equalID(row[parentIDName], parentID) {
			temp := copyMap(row)
			child := LinearToTree(data, subKey, idName, parentIDName, row[idName])
			temp[subKey] = child
			tree = append(tree, temp)
		}
	}
	return tree
}

func DeptTree(data []map[string]any, parentID any) []map[string]any {
	return deptTreeLevel(data, parentID, 0)
}

func deptTreeLevel(data []map[string]any, parentID any, level int) []map[string]any {
	tree := make([]map[string]any, 0)
	for _, row := range data {
		if equalID(row["pid"], parentID) {
			temp := copyMap(row)
			temp["level"] = level
			temp["children"] = deptTreeLevel(data, row["id"], level+1)
			tree = append(tree, temp)
		}
	}
	return tree
}

func equalID(a, b any) bool {
	return toInt64(a) == toInt64(b)
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case uint:
		return int64(n)
	case uint32:
		return int64(n)
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case string:
		return int64(ParseInt(strings.TrimSpace(n)))
	case json.Number:
		iv, _ := n.Int64()
		return iv
	default:
		return 0
	}
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
