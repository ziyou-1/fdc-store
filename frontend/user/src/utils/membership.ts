import { amountToCents, centsToAmount } from './money.ts'

export type MembershipProfile = {
  membership_discount_rate?: number | string
  membership_expires_at?: string | null
}

export function isMembershipActive(profile?: MembershipProfile | null, now = Date.now()) {
  if (!profile?.membership_expires_at) return false
  const expires = Date.parse(profile.membership_expires_at)
  if (!Number.isFinite(expires) || expires <= now) return false
  const rate = Number(profile.membership_discount_rate)
  return Number.isFinite(rate) && rate > 0 && rate < 100
}

export function discountOffN(rate: number, locale: string) {
  if (!Number.isFinite(rate)) return 0
  if (String(locale).startsWith('en')) return 100 - rate
  return rate / 10
}

export function purchasedMembershipAmount(
  base: unknown,
  profile?: MembershipProfile | null,
  now = Date.now(),
): string | null {
  if (!isMembershipActive(profile, now)) return null
  const rate = Number(profile?.membership_discount_rate)
  const cents = amountToCents(base)
  if (cents === null || cents <= 0) return null
  const memberCents = Math.round((cents * rate) / 100)
  if (memberCents >= cents) return null
  return centsToAmount(memberCents)
}
