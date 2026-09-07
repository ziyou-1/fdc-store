package jsonmap

import (
	"reflect"
	"testing"
)

func TestJSONDatabaseRoundTrip(t *testing.T) {
	original := JSON{
		"enabled": true,
		"name":    "storefront",
		"nested":  map[string]interface{}{"count": float64(2)},
	}

	encoded, err := original.Value()
	if err != nil {
		t.Fatalf("encode JSON value: %v", err)
	}

	var decoded JSON
	if err := decoded.Scan(encoded); err != nil {
		t.Fatalf("scan JSON value: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("JSON round trip mismatch\nwant: %#v\ngot:  %#v", original, decoded)
	}
}

func TestJSONScanNilCreatesEmptyObject(t *testing.T) {
	value := JSON{"stale": true}
	if err := value.Scan(nil); err != nil {
		t.Fatalf("scan nil JSON: %v", err)
	}
	if value == nil || len(value) != 0 {
		t.Fatalf("scan nil should create an empty object, got %#v", value)
	}
}

func TestJSONNilDatabaseValueStaysNull(t *testing.T) {
	var value JSON
	encoded, err := value.Value()
	if err != nil {
		t.Fatalf("encode nil JSON: %v", err)
	}
	if encoded != nil {
		t.Fatalf("nil JSON should produce a NULL driver value, got %#v", encoded)
	}
}

// SQLite 把 JSON 列以 TEXT 存放时读回的是 string 而不是 []byte。
// 早先的实现只认 []byte 并在不匹配时静默 return nil，导致整列内容无声变空。
func TestJSONScanAcceptsStringSource(t *testing.T) {
	want := JSON{"zh-CN": "买到的账号怎么激活？", "en-US": "How do I activate?"}

	var decoded JSON
	if err := decoded.Scan(`{"zh-CN":"买到的账号怎么激活？","en-US":"How do I activate?"}`); err != nil {
		t.Fatalf("scan string source: %v", err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("string source mismatch: want %#v, got %#v", want, decoded)
	}
}

// 无法识别的类型必须报错，而不是留下一个空 map 让调用方以为"这条数据本来就没内容"。
func TestJSONScanRejectsUnsupportedSource(t *testing.T) {
	var decoded JSON
	if err := decoded.Scan(42); err == nil {
		t.Fatal("unsupported source must fail loudly instead of silently yielding an empty object")
	}
}

func TestJSONScanReportsMalformedPayload(t *testing.T) {
	for name, source := range map[string]interface{}{
		"bytes":  []byte("{not json"),
		"string": "{not json",
	} {
		t.Run(name, func(t *testing.T) {
			var decoded JSON
			if err := decoded.Scan(source); err == nil {
				t.Fatal("malformed JSON must return an error")
			}
		})
	}
}
