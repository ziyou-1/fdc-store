export function referralInviteURL(origin: string, code: string): string {
  const trimmed = code.trim()
  if (trimmed === '') {
    throw new Error('invite code required')
  }
  return `${origin.replace(/\/$/, '')}/?aff=${encodeURIComponent(trimmed)}`
}
