package agiso

import (
	"strings"
	"testing"
)

// 官方文档给出了这组参数拼接后的完整中间串，是唯一可复现的权威样例
// （最终 MD5 复现不了，因为文档里的 AppSecret 被打码成星号）。
// 拼接顺序是整个签名里最容易写错的部分，这里直接钉死。
func TestBuildRequestPayloadMatchesOfficialExample(t *testing.T) {
	const secret = "SECRET"
	params := map[string]string{
		"modifyTimeStart": "2016-07-13 10:44:30",
		"pageNo":          "1",
		"pageSize":        "20",
		"timestamp":       "1468476350",
	}
	want := secret + "modifyTimeStart2016-07-13 10:44:30pageNo1pageSize20timestamp1468476350" + secret
	if got := buildRequestPayload(secret, params); got != want {
		t.Fatalf("拼接串与官方样例不符\n got=%s\nwant=%s", got, want)
	}
}

// 官方常见问题明确警告：参数排序不要忽略大小写。
// ASCII 序里大写字母全部排在小写字母之前，若误用大小写不敏感排序，签名会静默失败。
func TestSortIsCaseSensitive(t *testing.T) {
	params := map[string]string{"b": "1", "A": "2", "a": "3", "B": "4"}
	got := buildRequestPayload("", params)
	if want := "A2B4a3b1"; got != want {
		t.Fatalf("排序未按 ASCII 区分大小写\n got=%s\nwant=%s", got, want)
	}
}

// sign 自身不能参与签名，否则重复签名时结果会漂移。
func TestSignExcludesSignParam(t *testing.T) {
	base := map[string]string{"a": "1", "timestamp": "2"}
	withSign := map[string]string{"a": "1", "timestamp": "2", "sign": "whatever"}
	if SignRequest("s", base) != SignRequest("s", withSign) {
		t.Fatal("sign 参数不应参与签名计算")
	}
}

// 推送签名与调用签名是两套算法。
//
// 注意一个反直觉之处：只有 json 与 timestamp 两个参数时，两者结果**恰好相同**——
// 因为 ASCII 序里 json 本来就排在 timestamp 之前，与推送的固定顺序重合。
// 这个巧合正是危险所在：只拿两参数的用例去验证，会误以为可以复用同一个函数。
// 一旦多出任何一个排在 json 之前的参数，两者立刻分叉。
func TestPushSignIsNotGeneralRequestSign(t *testing.T) {
	const secret = "s"
	body := `{"CpkId":25}`
	ts := "1468476350"

	// 推送签名固定 json 在前、timestamp 在后，且完全不排序
	want := md5Hex(secret + "json" + body + "timestamp" + ts + secret)
	if got := SignPush(secret, body, ts); got != want {
		t.Fatalf("推送签名拼接顺序不对\n got=%s\nwant=%s", got, want)
	}

	// 多一个排在 json 之前的参数，调用签名会把它插到最前面；推送签名不受影响
	withExtra := SignRequest(secret, map[string]string{"aid": "9", "json": body, "timestamp": ts})
	if withExtra == SignPush(secret, body, ts) {
		t.Fatal("两套算法在存在额外参数时必须分叉，否则说明实现被错误地复用了")
	}
}

func TestVerifyPushIsCaseInsensitive(t *testing.T) {
	const secret = "s"
	body := `{"CpkId":25}`
	ts := "1"
	sig := SignPush(secret, body, ts)
	if !VerifyPush(secret, body, ts, strings.ToUpper(sig)) {
		t.Fatal("官方注明签名不区分大小写，大写形式应当通过")
	}
	if VerifyPush(secret, body, ts, "deadbeef") {
		t.Fatal("错误签名不应通过")
	}
}

// 官方常见问题里的坑：推送体经 form 编码后，日期中的空格会变成 +。
// 若解码时没把 + 还原成空格，签名必然对不上。这里固定住「参与验签的必须是解码后的原文」。
func TestVerifyPushNeedsDecodedBody(t *testing.T) {
	const secret = "s"
	decoded := `{"Created":"2020-01-01 15:00:00"}`
	raw := `{"Created":"2020-01-01+15:00:00"}` // 未还原 + 的错误形式
	ts := "1"
	sig := SignPush(secret, decoded, ts)

	if !VerifyPush(secret, decoded, ts, sig) {
		t.Fatal("解码后的原文应当验签通过")
	}
	if VerifyPush(secret, raw, ts, sig) {
		t.Fatal("未还原 + 的字符串不应通过——正是官方警告的那个陷阱")
	}
}
