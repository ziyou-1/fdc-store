import { userApi } from './client'

export const freeClaimAPI = {
    // 公开端点，但挂了可选鉴权：用 userApi 在已登录时带上 token，
    // 后端据此返回准确的 claimed；游客不带 token 同样正常返回。
    today: () => userApi.get('/public/free-claim/today'),
    claim: () => userApi.post('/free-claim'),
    records: (params?: { page?: number; page_size?: number }) =>
        userApi.get('/free-claim/records', { params }),
}
