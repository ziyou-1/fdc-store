package application

import (
	"errors"
	"strings"
	"sync"
)

// ErrTranslatorNotConfigured 表示当前没有可用的翻译器：翻译被关掉了，或者密钥没填。
//
// 它不是故障。调用方必须把它和"翻译失败"区分开：
// 没配置就跳过翻译照常落库，而不是把买家的消息打回去。
var ErrTranslatorNotConfigured = errors.New("support: translator not configured")

// TranslatorHolder 是可在运行时热换的翻译器槽位。
//
// 存在的理由是两个都必须解决的问题：
//
//  1. 翻译器原先在容器装配时构造一次就不再变，后台改了密钥必须重启进程才生效。
//  2. 装配处原先写的是 `if translator != nil { SetUserTextTranslator(...) }`，
//     启动时密钥为空就压根不注入，于是"运营第一次在后台填密钥"这个最常见的场景
//     哪怕热更了配置也毫无作用——因为业务侧那个函数字段自始至终是 nil。
//
// 所以这里改成无条件注入一个恒不为 nil 的间接层，密钥换不换由 holder 内部决定。
// endpoint 是密钥的派生值（:fx 结尾走免费版），换密钥必须整体重建翻译器，
// 只改字段会出现"用新密钥打旧端点"的错配，因此 Configure 直接替换整个实例。
type TranslatorHolder struct {
	mu         sync.RWMutex
	translator *DeepLTranslator
}

// NewTranslatorHolder 构造槽位。enabled 为 false 或密钥为空时槽位是空的，
// 此时 Translate 返回 ErrTranslatorNotConfigured。
func NewTranslatorHolder(enabled bool, authKey string) *TranslatorHolder {
	holder := &TranslatorHolder{}
	holder.Configure(enabled, authKey)
	return holder
}

// Configure 换掉当前翻译器。后台保存设置后由 ApplyRuntime 调用。
//
// 关掉开关或清空密钥都会把槽位置空——这条很重要：
// 少了它就会出现"后台把翻译关了，但旧密钥还在后台默默翻译并计费"的反向 bug。
func (h *TranslatorHolder) Configure(enabled bool, authKey string) {
	if h == nil {
		return
	}
	var next *DeepLTranslator
	if enabled {
		next = NewDeepLTranslator(authKey)
	}
	h.mu.Lock()
	h.translator = next
	h.mu.Unlock()
}

// Configured 报告当前是否有可用翻译器。
func (h *TranslatorHolder) Configured() bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.translator != nil
}

// Translate 按当前密钥翻译。未配置时返回 ErrTranslatorNotConfigured。
//
// 读锁只用来取出翻译器指针，网络请求在锁外发起：
// DeepL 单次调用有 10 秒超时，握着锁打网络会让后台保存设置一并卡住。
func (h *TranslatorHolder) Translate(text string) (string, error) {
	if h == nil {
		return "", ErrTranslatorNotConfigured
	}
	h.mu.RLock()
	translator := h.translator
	h.mu.RUnlock()
	if translator == nil {
		return "", ErrTranslatorNotConfigured
	}
	return translator.Translate(text)
}

// TranslateBatchTo 按当前密钥批量翻译。未配置时返回 ErrTranslatorNotConfigured。
//
// 写法必须和 Translate 一致：读锁内只取指针，网络请求在锁外发。
// 握着锁打网络会把后台"保存翻译设置"这个请求一起卡住——
// DeepL 单次超时 10 秒，批量还可能切成好几批。
func (h *TranslatorHolder) TranslateBatchTo(texts []string, targetLang, sourceLang, tagHandling string) ([]string, error) {
	if h == nil {
		return nil, ErrTranslatorNotConfigured
	}
	h.mu.RLock()
	translator := h.translator
	h.mu.RUnlock()
	if translator == nil {
		return nil, ErrTranslatorNotConfigured
	}
	return translator.TranslateBatchTo(texts, targetLang, sourceLang, tagHandling)
}

// HTMLTagHandling 暴露"富文本要用的 tag_handling 取值"，
// 免得调用方自己写字符串字面量、写错了还不报错。
func HTMLTagHandling() string { return deeplTagHandlingHTML }

// VerifyAuthKey 用给定密钥试译一小段文本，用来在保存前验证密钥是否可用。
//
// 翻译在业务侧是 fail-closed 的（译不出来买家就发不出客服消息），
// 所以密钥填错的代价是整站客服文本消息不可用。给运营一个存之前自查的探针，
// 比让他们把错密钥存进去再从买家投诉里发现要好。
func VerifyAuthKey(authKey string) error {
	translator := NewDeepLTranslator(authKey)
	if translator == nil {
		return ErrTranslatorNotConfigured
	}
	translated, err := translator.Translate("ping")
	if err != nil {
		return err
	}
	if strings.TrimSpace(translated) == "" {
		return errors.New("deepl returned empty translation")
	}
	return nil
}
