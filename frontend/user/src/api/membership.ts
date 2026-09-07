import { userApi } from './client'
import type { MembershipPurchasePayload } from './types'

export const membershipAPI = {
    purchase: (data: MembershipPurchasePayload) => userApi.post('/membership/purchase', data),
}
