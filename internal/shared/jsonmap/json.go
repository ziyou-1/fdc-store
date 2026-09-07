// Package jsonmap provides the shared JSON object value used at persistence
// and transport boundaries. It is deliberately independent from GORM models
// and business modules so bounded contexts do not need to import a global
// models package merely to exchange arbitrary JSON objects.
package jsonmap

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON is a JSON object with database/sql serialization support.
type JSON map[string]interface{}

// Value implements driver.Valuer.
func (value JSON) Value() (driver.Value, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

// Scan implements sql.Scanner.
//
// 驱动对 JSON 列的返回类型不统一：应用自己写入时 Value() 交出 []byte（SQLite 落成
// BLOB、读回仍是 []byte），但同一列若以 TEXT 形式存在（外部工具导入、旧迁移建成
// TEXT），SQLite 读回的是 string。因此两种都必须接受——早先只认 []byte 且在不匹配时
// 静默 return nil，会让整列 JSON 无声变空而不报任何错。
func (value *JSON) Scan(source interface{}) error {
	if source == nil {
		*value = make(JSON)
		return nil
	}
	switch data := source.(type) {
	case []byte:
		return json.Unmarshal(data, value)
	case string:
		return json.Unmarshal([]byte(data), value)
	default:
		return fmt.Errorf("jsonmap: cannot scan %T into JSON", source)
	}
}
