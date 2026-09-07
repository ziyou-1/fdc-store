import { userApi } from './client'
import type { PointsRechargePayload } from './types'

export const pointsAPI = {
    account: () => userApi.get('/points'),
    config: () => userApi.get('/points/config'),
    invite: () => userApi.get('/points/invite'),
    transactions: (params?: { page?: number; page_size?: number; type?: string; direction?: string }) =>
        userApi.get('/points/transactions', { params }),
    checkinStatus: () => userApi.get('/points/checkin'),
    checkin: () => userApi.post('/points/checkin'),
    // 充值渠道复用钱包的 walletAPI.getPaymentChannels(amount)，后端是同一条校验链路。
    recharge: (data: PointsRechargePayload) => userApi.post('/points/recharge', data),
    recharges: (params?: { page?: number; page_size?: number; status?: string }) =>
        userApi.get('/points/recharges', { params }),
}
