package jsonslice

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// scanJSONSource 归一化驱动返回的 JSON 载荷。
// 应用自己写入时 Value() 交出 []byte（SQLite 落成 BLOB），但同一列若以 TEXT 存在
// 则读回 string，两种都必须接受；未知类型必须报错而不是静默留下空切片。
func scanJSONSource(value interface{}, target interface{}) error {
	switch data := value.(type) {
	case []byte:
		return json.Unmarshal(data, target)
	case string:
		return json.Unmarshal([]byte(data), target)
	default:
		return fmt.Errorf("jsonslice: cannot scan %T", value)
	}
}

// Strings is a JSON-backed string slice suitable for SQL value/scan contracts.
type Strings []string

func (items Strings) Value() (driver.Value, error) {
	if items == nil {
		return nil, nil
	}
	return json.Marshal(items)
}

func (items *Strings) Scan(value interface{}) error {
	if value == nil {
		*items = Strings{}
		return nil
	}
	return scanJSONSource(value, items)
}

// Uints is a JSON-backed unsigned-integer slice suitable for SQL value/scan contracts.
type Uints []uint

func (items Uints) Value() (driver.Value, error) {
	if items == nil {
		return nil, nil
	}
	return json.Marshal(items)
}

func (items *Uints) Scan(value interface{}) error {
	if value == nil {
		*items = Uints{}
		return nil
	}
	return scanJSONSource(value, items)
}
