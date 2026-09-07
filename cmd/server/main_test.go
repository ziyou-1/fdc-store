package main

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
)

var bannerANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// bannerLinks 提取横幅里的链接，剥掉 ANSI 颜色码并按空白切分。
// 断言链接本身而非"标签 + 对齐空白"的整行字面量，横幅排版微调就不会连带弄红 CI。
func bannerLinks(banner string) []string {
	var links []string
	for _, field := range strings.Fields(bannerANSIPattern.ReplaceAllString(banner, "")) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			links = append(links, field)
		}
	}
	return links
}

func TestWriteStartupBannerCarriesNoExternalLinks(t *testing.T) {
	var output strings.Builder
	writeStartupBanner(&output)

	banner := output.String()

	// 本项目已改名并去掉上游品牌，横幅不得再出现任何外链：
	// 组织地址是主仓地址的前缀，所以用整 token 比较而不是子串匹配。
	if links := bannerLinks(banner); len(links) != 0 {
		t.Errorf("startup banner must not advertise external links, got %v", links)
	}

	for _, forbidden := range []string{"dujiao", "Dujiao", "DUJIAO", "t.me/"} {
		if strings.Contains(banner, forbidden) {
			t.Errorf("startup banner still mentions upstream branding: %s", forbidden)
		}
	}

	if !strings.Contains(banner, "Keyvo") {
		t.Error("startup banner should announce the current product name")
	}
}

func TestWeakRuntimeSecretNamesCoversEveryRootSecret(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "change-me-32-byte-secret-key!!"},
		JWT:     config.JWTConfig{SecretKey: "your-secret-key-change-in-production-please"},
		UserJWT: config.JWTConfig{SecretKey: "user-change-me-in-production"},
	}

	want := []string{"app.secret_key", "jwt.secret", "user_jwt.secret"}
	if got := weakRuntimeSecretNames(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("weak runtime secrets want %v got %v", want, got)
	}
}

func TestWeakRuntimeSecretNamesAcceptsStrongIndependentSecrets(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "2f8d164772cd4bbcaef8fa4ad19a2a26f7a15505"},
		JWT:     config.JWTConfig{SecretKey: "dd914407e55c4528a393fe522215c18f5fc8687b"},
		UserJWT: config.JWTConfig{SecretKey: "ca36df49b49446d2a9b2cac7f035d11574575b53"},
	}

	if got := weakRuntimeSecretNames(cfg); len(got) != 0 {
		t.Fatalf("strong runtime secrets reported as weak: %v", got)
	}
}

func TestWeakRuntimeSecretNamesRejectsReusedStrongSecrets(t *testing.T) {
	shared := "2f8d164772cd4bbcaef8fa4ad19a2a26f7a15505"
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: shared},
		JWT:     config.JWTConfig{SecretKey: shared},
		UserJWT: config.JWTConfig{SecretKey: "ca36df49b49446d2a9b2cac7f035d11574575b53"},
	}

	want := []string{"app.secret_key", "jwt.secret"}
	if got := weakRuntimeSecretNames(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("reused runtime secrets want %v got %v", want, got)
	}
}

func TestUnsafeBootstrapAdminPasswordRejectsDefaultsAndPolicyViolations(t *testing.T) {
	cfg := &config.Config{Security: config.SecurityConfig{PasswordPolicy: config.PasswordPolicyConfig{
		MinLength:     10,
		RequireUpper:  true,
		RequireLower:  true,
		RequireNumber: true,
	}}}

	for _, password := range []string{"admin123", "alllowercase1", "NOLOWERCASE1", "NoNumberHere"} {
		if !unsafeBootstrapAdminPassword(cfg, password) {
			t.Errorf("expected bootstrap password %q to be rejected", password)
		}
	}
	if unsafeBootstrapAdminPassword(cfg, "StrongBootstrap123") {
		t.Fatal("expected strong bootstrap password to be accepted")
	}
	if unsafeBootstrapAdminPassword(cfg, "") {
		t.Fatal("empty bootstrap password should keep the skip-initialization behavior")
	}
}
