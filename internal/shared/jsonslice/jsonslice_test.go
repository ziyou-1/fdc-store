package jsonslice

import (
	"reflect"
	"testing"
)

func TestStringsValueAndScan(t *testing.T) {
	value, err := (Strings{"alpha", "beta"}).Value()
	if err != nil {
		t.Fatalf("encode strings: %v", err)
	}
	var decoded Strings
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("scan strings: %v", err)
	}
	if !reflect.DeepEqual(decoded, Strings{"alpha", "beta"}) {
		t.Fatalf("unexpected strings: %#v", decoded)
	}
	if err := decoded.Scan(nil); err != nil || decoded == nil || len(decoded) != 0 {
		t.Fatalf("nil scan must yield an empty non-nil slice: %#v err=%v", decoded, err)
	}
}

func TestUintsValueAndScan(t *testing.T) {
	value, err := (Uints{1, 2}).Value()
	if err != nil {
		t.Fatalf("encode uints: %v", err)
	}
	var decoded Uints
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("scan uints: %v", err)
	}
	if !reflect.DeepEqual(decoded, Uints{1, 2}) {
		t.Fatalf("unexpected uints: %#v", decoded)
	}
}

// SQLite 以 TEXT 存放 JSON 列时读回的是 string；早先只认 []byte 且静默放弃，
// 会让商品 images 之类的字段无声变成空数组。
func TestScanAcceptsStringSource(t *testing.T) {
	var decoded Strings
	if err := decoded.Scan(`["a","b"]`); err != nil {
		t.Fatalf("scan string source: %v", err)
	}
	if !reflect.DeepEqual(decoded, Strings{"a", "b"}) {
		t.Fatalf("unexpected strings: %#v", decoded)
	}

	var ids Uints
	if err := ids.Scan(`[3,7]`); err != nil {
		t.Fatalf("scan uint string source: %v", err)
	}
	if !reflect.DeepEqual(ids, Uints{3, 7}) {
		t.Fatalf("unexpected uints: %#v", ids)
	}
}

func TestScanRejectsUnsupportedSource(t *testing.T) {
	var decoded Strings
	if err := decoded.Scan(42); err == nil {
		t.Fatal("unsupported source must fail loudly instead of yielding an empty slice")
	}
	var ids Uints
	if err := ids.Scan(struct{}{}); err == nil {
		t.Fatal("unsupported source must fail loudly for Uints too")
	}
}
