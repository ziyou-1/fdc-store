package agiso

import (
	"errors"
	"fmt"
)

// 哨兵错误。调用方按类别决策，不要去比对 Error_Code 数字。
var (
	// ErrTokenInvalid 商家授权失效。AccessToken 的有效期与商家购买 Agiso 软件的时长一致，
	// 商家一旦停止续费，所有接卡券库的商品会立刻发不出货，必须告警而不是静默重试。
	ErrTokenInvalid = errors.New("agiso: access token invalid")

	// ErrInsufficientDeposit 开放平台预存款不足，接口被限制调用（错误码 16）。
	// 与 ErrTokenInvalid 同级：余额见底 = 全站停止发卡，必须告警。
	ErrInsufficientDeposit = errors.New("agiso: insufficient deposit")

	// ErrRateLimited 触发调用频率限制（官方配额 20 次/秒）。可退避重试。
	ErrRateLimited = errors.New("agiso: rate limited")

	// ErrPartialFailure 部分失败（错误码 6）。提卡数量大于 1 时可能只提到一部分，
	// 既不能当成功发货，也不能当全失败回滚，必须转人工。
	ErrPartialFailure = errors.New("agiso: partial failure")

	// ErrSignInvalid 验签失败，通常是本机时间偏差过大或参数排序有误。
	ErrSignInvalid = errors.New("agiso: signature invalid")

	// ErrNoData 没有数据（错误码 11 / 114）。
	ErrNoData = errors.New("agiso: no data")

	// ErrBusiness 其余业务异常，不可重试。
	ErrBusiness = errors.New("agiso: business error")

	// ErrTransient 超时一类的瞬时错误，可重试。
	ErrTransient = errors.New("agiso: transient error")
)

// APIError 保留原始错误码与 RequestId，便于对着 Agiso 后台排查。
type APIError struct {
	Code       int
	Msg        string
	RequestID  string
	AllowRetry *bool
	kind       error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("agiso: code=%d msg=%q request_id=%s", e.Code, e.Msg, e.RequestID)
}

func (e *APIError) Unwrap() error { return e.kind }

// Retryable 报告这次失败是否可以原样重试。
//
// 服务端显式给了 AllowRetry 就以它为准——这是 Agiso 对该次调用的权威判断；
// 没给时才回落到按错误码分类。提卡接口带幂等键 handPickOrderId，
// 重试不会重复扣卡，所以这里可以放心让瞬时错误重试。
func (e *APIError) Retryable() bool {
	if e.AllowRetry != nil {
		return *e.AllowRetry
	}
	return errors.Is(e.kind, ErrRateLimited) || errors.Is(e.kind, ErrTransient)
}

// classify 把 Agiso 的错误码归到哨兵错误。
// 只映射与本集成相关的码；淘宝/拼多多等平台专有的码一律归为业务异常。
func classify(code int) error {
	switch code {
	case 1, 8, 17, 18, 28:
		// 1 无效AccessToken / 8 授权失败 / 17 应用已被禁用 / 18 业务站点未授权 / 28 无API接口权限
		return ErrTokenInvalid
	case 16:
		return ErrInsufficientDeposit
	case 2, 25:
		return ErrRateLimited
	case 4:
		return ErrSignInvalid
	case 5:
		return ErrBusiness
	case 6:
		return ErrPartialFailure
	case 11, 114:
		return ErrNoData
	case 13:
		return ErrTransient
	default:
		return ErrBusiness
	}
}
