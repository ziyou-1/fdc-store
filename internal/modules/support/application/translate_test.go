package application

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewDeepLTranslatorEmptyKeyIsNil(t *testing.T) {
	if NewDeepLTranslator("  ") != nil {
		t.Fatal("empty auth key must not construct a translator")
	}
}

func TestDeepLTranslateURLSelectsHostByKeySuffix(t *testing.T) {
	if got := deeplTranslateURL("abc:fx"); got != deeplTranslateFree {
		t.Fatalf("free key host = %s", got)
	}
	if got := deeplTranslateURL("abc"); got != deeplTranslatePro {
		t.Fatalf("pro key host = %s", got)
	}
}

func TestParseDeepLTranslationReadsFirstText(t *testing.T) {
	got, err := parseDeepLTranslation([]byte(`{"translations":[{"detected_source_language":"ID","text":"您好"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != "您好" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDeepLTranslationRejectsEmptyPayloads(t *testing.T) {
	cases := []string{
		`{}`,
		`{"translations":[]}`,
		`{"translations":[{"text":"  "}]}`,
		`not-json`,
	}
	for _, body := range cases {
		if _, err := parseDeepLTranslation([]byte(body)); err == nil {
			t.Fatalf("expected error for %s", body)
		}
	}
}

func TestDeepLTranslatorTranslatePostsJSON(t *testing.T) {
	var gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translations":[{"text":"您好，抱歉，我先取消一下哦"}]}`))
	}))
	defer server.Close()

	translator := &DeepLTranslator{
		authKey:  "test-key",
		endpoint: server.URL,
		client:   server.Client(),
	}
	got, err := translator.Translate("Kak maaf saya batalkan dulu ya")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if got != "您好，抱歉，我先取消一下哦" {
		t.Fatalf("got %q", got)
	}
	if gotAuth != "DeepL-Auth-Key test-key" {
		t.Fatalf("auth header %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"target_lang":"ZH-HANS"`) {
		t.Fatalf("request body %s", gotBody)
	}
}

func TestDeepLTranslatorTranslateSurfacesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Quota exceeded"}`))
	}))
	defer server.Close()

	translator := &DeepLTranslator{
		authKey:  "test-key",
		endpoint: server.URL,
		client:   server.Client(),
	}
	_, err := translator.Translate("halo")
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "Quota exceeded") {
		t.Fatalf("expected status and DeepL message, got %v", err)
	}
}
