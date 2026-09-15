package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 批量翻译必须一次请求带上全部文本，并按同序返回。
// 逐条翻会让一次商品保存打十几次 DeepL，必然撞穿 10 秒超时。
func TestTranslateBatchToSendsOneRequestPerChunk(t *testing.T) {
	var requests int
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translations":[{"text":"satu"},{"text":"dua"},{"text":"tiga"}]}`))
	}))
	defer server.Close()

	translator := &DeepLTranslator{authKey: "k", endpoint: server.URL, client: server.Client()}
	got, err := translator.TranslateBatchTo([]string{"一", "二", "三"}, "ID", "ZH", "")
	if err != nil {
		t.Fatalf("batch translate: %v", err)
	}
	if requests != 1 {
		t.Fatalf("三条文本应当只发一次请求，实际发了 %d 次", requests)
	}
	if len(got) != 3 || got[0] != "satu" || got[2] != "tiga" {
		t.Fatalf("译文顺序或内容不对: %#v", got)
	}
	texts, _ := captured["text"].([]any)
	if len(texts) != 3 {
		t.Fatalf("请求体里应带 3 条文本，实得 %v", captured["text"])
	}
}

// 译文条数和原文对不上时必须整批失败。
// 继续用下去会把 A 字段的译文写到 B 字段上，那是静默的数据错乱。
func TestTranslateBatchToRejectsLengthMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"translations":[{"text":"satu"}]}`))
	}))
	defer server.Close()

	translator := &DeepLTranslator{authKey: "k", endpoint: server.URL, client: server.Client()}
	if _, err := translator.TranslateBatchTo([]string{"一", "二"}, "ID", "", ""); err == nil {
		t.Fatal("译文条数少于原文时应当报错")
	}
}

// 富文本字段必须带 tag_handling=html，否则 DeepL 会把
// <a href> 里的 URL 和 <img src> 的路径当正文一起翻掉。
func TestTranslateBatchToSendsTagHandlingForHTML(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"translations":[{"text":"<p>halo</p>"}]}`))
	}))
	defer server.Close()

	translator := &DeepLTranslator{authKey: "k", endpoint: server.URL, client: server.Client()}
	if _, err := translator.TranslateBatchTo([]string{"<p>你好</p>"}, "ID", "", deeplTagHandlingHTML); err != nil {
		t.Fatalf("translate: %v", err)
	}
	if captured["tag_handling"] != "html" {
		t.Fatalf("tag_handling = %v，应为 html", captured["tag_handling"])
	}

	// 纯文本字段不该带这个参数
	captured = nil
	if _, err := translator.TranslateBatchTo([]string{"你好"}, "ID", "", ""); err != nil {
		t.Fatalf("translate: %v", err)
	}
	if _, exists := captured["tag_handling"]; exists {
		t.Error("纯文本字段不应发送 tag_handling")
	}
}

// 超过 50 条要自动切批，且切批不能丢文本、不能乱序。
func TestChunkTextsSplitsByCountAndSize(t *testing.T) {
	texts := make([]string, 120)
	for i := range texts {
		texts[i] = fmt.Sprintf("t%d", i)
	}
	chunks := chunkTextsForDeepL(texts)
	if len(chunks) != 3 {
		t.Fatalf("120 条按 50 一批应切成 3 批，实得 %d", len(chunks))
	}
	var flat []string
	for _, c := range chunks {
		if len(c) > deeplMaxTextsPerRequest {
			t.Fatalf("有一批超过了 %d 条", deeplMaxTextsPerRequest)
		}
		flat = append(flat, c...)
	}
	if len(flat) != len(texts) {
		t.Fatalf("切批后条数变了: %d -> %d", len(texts), len(flat))
	}
	for i := range texts {
		if flat[i] != texts[i] {
			t.Fatalf("第 %d 条顺序错乱: %q != %q", i, flat[i], texts[i])
		}
	}

	// 字节数超限也要切，即便条数没到 50
	big := []string{strings.Repeat("a", 60*1024), strings.Repeat("b", 60*1024)}
	if got := len(chunkTextsForDeepL(big)); got != 2 {
		t.Fatalf("两条各 60KiB 应切成 2 批，实得 %d", got)
	}

	// 单条就超限时让它独占一批，而不是被丢掉
	huge := []string{strings.Repeat("a", 200*1024)}
	chunks = chunkTextsForDeepL(huge)
	if len(chunks) != 1 || len(chunks[0]) != 1 {
		t.Fatalf("单条超限应独占一批，实得 %#v", chunks)
	}
}

// 切批之后仍要一次不落地把每批都发出去。
func TestTranslateBatchToSendsEveryChunk(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body struct {
			Text []string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		items := make([]string, 0, len(body.Text))
		for range body.Text {
			items = append(items, `{"text":"x"}`)
		}
		_, _ = w.Write([]byte(`{"translations":[` + strings.Join(items, ",") + `]}`))
	}))
	defer server.Close()

	texts := make([]string, 120)
	for i := range texts {
		texts[i] = fmt.Sprintf("t%d", i)
	}
	translator := &DeepLTranslator{authKey: "k", endpoint: server.URL, client: server.Client()}
	got, err := translator.TranslateBatchTo(texts, "ID", "", "")
	if err != nil {
		t.Fatalf("batch translate: %v", err)
	}
	if requests != 3 {
		t.Fatalf("应发 3 次请求，实得 %d", requests)
	}
	if len(got) != 120 {
		t.Fatalf("译文条数 = %d，应为 120", len(got))
	}
}

// 空入参不该发请求出去。
func TestTranslateBatchToSkipsEmptyInput(t *testing.T) {
	translator := &DeepLTranslator{authKey: "k", endpoint: "http://127.0.0.1:1", client: http.DefaultClient}
	got, err := translator.TranslateBatchTo(nil, "ID", "", "")
	if err != nil || got != nil {
		t.Fatalf("空入参应直接返回，实得 %v / %v", got, err)
	}
}
