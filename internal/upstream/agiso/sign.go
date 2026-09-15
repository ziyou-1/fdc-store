package agiso

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strings"
)

// Agiso（91卡券）有两套互不相同的签名算法，务必分清：
//
//	SignRequest —— 主动调用 API 时用。参数名按 ASCII 升序排列后拼 name+value，前后各加一次
//	               AppSecret 再 MD5。排序区分大小写（官方常见问题明确警告过这一点）。
//	SignPush    —— 校验 Agiso 推送过来的通知时用。固定 json 在前、timestamp 在后，不排序。
//
// 两者只差一点点，写成同一个函数复用必然出错，因此这里刻意分开且不共享拼接逻辑。

// SignRequest 生成主动调用 API 的 sign 参数。
// params 不应包含 sign 本身；调用方负责把 timestamp 放进来。
func SignRequest(appSecret string, params map[string]string) string {
	return md5Hex(buildRequestPayload(appSecret, params))
}

// buildRequestPayload 返回参与 MD5 的那个中间串。
// 单独拆出来是为了让测试能直接钉住拼接顺序——官方文档给的示例只有中间串可复现，
// 最终哈希因为 AppSecret 被打码而无法复算。
func buildRequestPayload(appSecret string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	// 按 ASCII 码序，区分大小写：sort.Strings 正是逐字节比较，符合要求。
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(appSecret)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(appSecret)
	return b.String()
}

// SignPush 生成/校验推送通知的签名。
// 顺序固定为 json、timestamp，且不参与排序——这与 SignRequest 完全不同。
func SignPush(appSecret, jsonBody, timestamp string) string {
	var b strings.Builder
	b.WriteString(appSecret)
	b.WriteString("json")
	b.WriteString(jsonBody)
	b.WriteString("timestamp")
	b.WriteString(timestamp)
	b.WriteString(appSecret)
	return md5Hex(b.String())
}

// VerifyPush 校验推送签名。大小写不敏感——官方示例中注明「不区分大小写」。
//
// 注意 jsonBody 必须是 url 解码后的原文：Agiso 推送用 form 编码，日期里的空格会被编成 +，
// 若解码时没有把 + 还原成空格，签名必然对不上（官方常见问题里专门列了这一条）。
// Go 的 http.Request.FormValue / ParseForm 会正确处理，手工解析 body 时才需要留意。
func VerifyPush(appSecret, jsonBody, timestamp, got string) bool {
	return strings.EqualFold(SignPush(appSecret, jsonBody, timestamp), strings.TrimSpace(got))
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
