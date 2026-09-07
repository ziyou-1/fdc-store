package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	deeplTargetLang     = "ZH-HANS"
	deeplTranslatePro   = "https://api.deepl.com/v2/translate"
	deeplTranslateFree  = "https://api-free.deepl.com/v2/translate"
	deeplRequestTimeout = 10 * time.Second
)

// DeepLTranslator 调用 DeepL /v2/translate，把买家原文译成简体中文。
type DeepLTranslator struct {
	authKey  string
	endpoint string
	client   *http.Client
}

// NewDeepLTranslator 空密钥表示未配置，返回 nil。
func NewDeepLTranslator(authKey string) *DeepLTranslator {
	key := strings.TrimSpace(authKey)
	if key == "" {
		return nil
	}
	return &DeepLTranslator{
		authKey:  key,
		endpoint: deeplTranslateURL(key),
		client:   &http.Client{Timeout: deeplRequestTimeout},
	}
}

func deeplTranslateURL(authKey string) string {
	if strings.HasSuffix(authKey, ":fx") {
		return deeplTranslateFree
	}
	return deeplTranslatePro
}

// Translate 把一段文本译成简体中文。
func (t *DeepLTranslator) Translate(text string) (string, error) {
	if t == nil {
		return "", fmt.Errorf("deepl translator is nil")
	}
	payload, err := json.Marshal(map[string]any{
		"text":        []string{text},
		"target_lang": deeplTargetLang,
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodPost, t.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "DeepL-Auth-Key "+t.authKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "dujiao-next/support")

	client := t.client
	if client == nil {
		client = &http.Client{Timeout: deeplRequestTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("deepl status %d: %s", response.StatusCode, deeplErrorMessage(body))
	}
	return parseDeepLTranslation(body)
}

func parseDeepLTranslation(body []byte) (string, error) {
	var parsed struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("deepl response: %w", err)
	}
	if len(parsed.Translations) == 0 {
		return "", fmt.Errorf("deepl response: empty translations")
	}
	text := strings.TrimSpace(parsed.Translations[0].Text)
	if text == "" {
		return "", fmt.Errorf("deepl response: empty text")
	}
	return text, nil
}

func deeplErrorMessage(body []byte) string {
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && strings.TrimSpace(parsed.Message) != "" {
		return strings.TrimSpace(parsed.Message)
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "empty body"
	}
	return trimmed
}
