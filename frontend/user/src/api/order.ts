import { userApi } from './client'
import type { CreatePaymentPayload } from './types'

export const userOrderAPI = {
    preview: (data: any) => userApi.post('/orders/preview', data),
    getPaymentChannels: (data: any) => userApi.post('/order/payment-channels', data),
    create: (data: any) => userApi.post('/orders', data),
    createAndPay: (data: any) => userApi.post('/orders/create-and-pay', data),
    list: (params?: any) => userApi.get('/orders', { params }),
    stats: (params?: any) => userApi.get('/orders/stats', { params }),
    detail: (orderNo: string, options?: any) => userApi.get(`/orders/${encodeURIComponent(orderNo)}`, options),
    cancel: (orderNo: string) => userApi.post(`/orders/${encodeURIComponent(orderNo)}/cancel`),
    downloadFulfillment: (orderNo: string) => userApi.get(`/orders/${encodeURIComponent(orderNo)}/fulfillment/download`, { blob: true }),
    getPendingManualForm: (orderNo: string) => userApi.get(`/orders/${encodeURIComponent(orderNo)}/manual-form`),
    submitManualForm: (orderNo: string, data: { submissions?: Record<string, any>; data?: Record<string, any> }) =>
        userApi.post(`/orders/${encodeURIComponent(orderNo)}/manual-form`, data),
}

export const paymentAPI = {
    create: (data: CreatePaymentPayload) => userApi.post('/payments', data),
    capture: (id: number) => userApi.post(`/payments/${id}/capture`),
    latest: (params: any) => userApi.get('/payments/latest', { params, silentBusinessError: true }),
}
