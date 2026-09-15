package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// DeepL 单次请求最多 50 条文本。
	deeplMaxTextsPerRequest = 50
	// 请求体大小上限，保守取 64 KiB（DeepL 免费版是 128 KiB）。
	// 留这么大余量是因为估算用的是原文字节数，而实际发出去的是 JSON：
	// 富文本 HTML 里的每个引号都会膨胀成 \"，加上 text 数组与其余字段的结构开销，
	// 实际请求体会明显大于原文之和。宁可多切一批，也不要撞 413。
	deeplMaxRequestBytes = 64 * 1024
	// tag_handling=html 让 DeepL 认得 HTML 标签，只翻标签之间的文本。
	// 商品详情由 tiptap 产出，是 HTML 不是 Markdown；不开这个参数会把
	// <a href="..."> 里的 URL、<img src="..."> 的路径当正文一起翻掉。
	deeplTagHandlingHTML = "html"
)

// TranslateBatchTo 一次把多条文本译成同一种语言，返回与入参等长、顺序一致的译文数组。
//
// 批量是必需的而不是优化：一个商品有 6 类多语言文本 × 3 门目标语言 = 最多 18 次调用，
// 而 DeepL 单次超时就是 10 秒，后台保存接口的前端超时也是 10 秒——逐条翻必然超时。
// 打包成"每种目标语言一次请求"之后是 3 次，可以并发发出去。
//
// tagHandling 传 deeplTagHandlingHTML 用于富文本字段；纯文本字段传空串。
func (t *DeepLTranslator) TranslateBatchTo(texts []string, targetLang, sourceLang, tagHandling string) ([]string, error) {
	if t == nil {
		return nil, fmt.Errorf("deepl translator is nil")
	}
	if strings.TrimSpace(targetLang) == "" {
		return nil, fmt.Errorf("deepl: target language is required")
	}
	if len(texts) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(texts))
	for _, chunk := range chunkTextsForDeepL(texts) {
		translated, err := t.translateChunk(chunk, targetLang, sourceLang, tagHandling)
		if err != nil {
			return nil, err
		}
		// DeepL 保证 translations 与入参同序等长。长度对不上说明响应不可信，
		// 继续往下用会把 A 商品的译文写到 B 字段上，宁可整批失败。
		if len(translated) != len(chunk) {
			return nil, fmt.Errorf("deepl response: expected %d translations, got %d", len(chunk), len(translated))
		}
		out = append(out, translated...)
	}
	return out, nil
}

// chunkTextsForDeepL 按条数与字节数双重上限切批。
// 单条就超限时让它独占一批——DeepL 会给出明确报错，好过我们静默丢掉它。
func chunkTextsForDeepL(texts []string) [][]string {
	chunks := make([][]string, 0, 1)
	current := make([]string, 0, deeplMaxTextsPerRequest)
	currentBytes := 0

	for _, text := range texts {
		size := len(text)
		tooMany := len(current) >= deeplMaxTextsPerRequest
		tooBig := len(current) > 0 && currentBytes+size > deeplMaxRequestBytes
		if tooMany || tooBig {
			chunks = append(chunks, current)
			current = make([]string, 0, deeplMaxTextsPerRequest)
			currentBytes = 0
		}
		current = append(current, text)
		currentBytes += size
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

func (t *DeepLTranslator) translateChunk(texts []string, targetLang, sourceLang, tagHandling string) ([]string, error) {
	requestBody := map[string]any{
		"text":        texts,
		"target_lang": targetLang,
	}
	if trimmed := strings.TrimSpace(sourceLang); trimmed != "" {
		requestBody["source_lang"] = trimmed
	}
	if trimmed := strings.TrimSpace(tagHandling); trimmed != "" {
		requestBody["tag_handling"] = trimmed
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, t.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "DeepL-Auth-Key "+t.authKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "dujiao-next/translate")

	client := t.client
	if client == nil {
		client = &http.Client{Timeout: deeplRequestTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("deepl status %d: %s", response.StatusCode, deeplErrorMessage(body))
	}
	return parseDeepLTranslations(body)
}

// parseDeepLTranslations 取出全部译文，保持顺序。
// 单条为空不在这里报错——批量场景下调用方需要知道"哪一条空了"，
// 整批失败会连带丢掉本来译好的其它字段。
func parseDeepLTranslations(body []byte) ([]string, error) {
	var parsed struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("deepl response: %w", err)
	}
	if len(parsed.Translations) == 0 {
		return nil, fmt.Errorf("deepl response: empty translations")
	}
	out := make([]string, 0, len(parsed.Translations))
	for _, item := range parsed.Translations {
		out = append(out, strings.TrimSpace(item.Text))
	}
	return out, nil
}
