package application

import (
	"github.com/dujiao-next/internal/constants"
)

// 后台对支付记录的生命周期操作。刻意不放进 payment_service.go：
// 那个文件按架构守卫只承载构造、查询与端口注入，变更操作一律拆出来。

// DeletePayment 软删除一条支付记录。
//
// 成功的支付是买家真的付过钱的凭证，删掉会让订单的已付状态失去依据，
// 也没法再跟网关对账，因此一律拒绝——要处理已成功的支付应该走退款流程。
// 其余状态（initiated/pending/failed/expired）多是弃单留下的噪音，允许清理。
// 删除是软删除：行保留在库里可追溯，只是不再出现在后台列表与统计中。
func (s *PaymentService) DeletePayment(id uint) error {
	if id == 0 {
		return ErrPaymentInvalid
	}
	payment, err := s.paymentRepo.GetByID(id)
	if err != nil {
		return ErrPaymentUpdateFailed
	}
	if payment == nil {
		return ErrPaymentNotFound
	}
	if payment.Status == constants.PaymentStatusSuccess {
		return ErrPaymentDeleteNotAllowed
	}
	return s.paymentRepo.Delete(id)
}
