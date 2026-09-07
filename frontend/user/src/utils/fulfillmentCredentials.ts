export type FulfillmentCredential = {
  account: string
  password: string
}

export type FulfillmentCredentialView = {
  credentials: FulfillmentCredential[]
  extraLines: string[]
}

const ACCOUNT_KEY = /^(account|username|user|email|login|账号|帳號|用户名|使用者)$/i
const PASSWORD_KEY = /^(password|pass|pwd|secret|密码|密碼)$/i

export function parseSecretLine(line: string): FulfillmentCredential | null {
  const text = String(line || '').trim()
  if (!text || /^https?:\/\//i.test(text)) {
    return null
  }

  const dashed = text.split('----')
  if (dashed.length === 2) {
    const account = String(dashed[0] || '').trim()
    const password = String(dashed[1] || '').trim()
    if (account && password) {
      return { account, password }
    }
  }

  if (/^\S+:\S+$/.test(text) && text.indexOf(':') === text.lastIndexOf(':')) {
    const colon = text.indexOf(':')
    return {
      account: text.slice(0, colon),
      password: text.slice(colon + 1),
    }
  }

  return null
}

export function parseFulfillmentCredentials(fulfillment: any): FulfillmentCredentialView {
  const credentials: FulfillmentCredential[] = []
  const extraLines: string[] = []
  const deliveryData = fulfillment?.delivery_data || fulfillment?.logistics

  if (deliveryData && typeof deliveryData === 'object') {
    const note = String(deliveryData.note || '').trim()
    if (note) {
      extraLines.push(note)
    }

    let pendingAccount = ''
    let pendingPassword = ''
    const flushPair = () => {
      if (pendingAccount && pendingPassword) {
        credentials.push({ account: pendingAccount, password: pendingPassword })
        pendingAccount = ''
        pendingPassword = ''
      }
    }

    const entries = Array.isArray(deliveryData.entries) ? deliveryData.entries : []
    for (const entry of entries) {
      const key = String(entry?.key || '').trim()
      const value = String(entry?.value || '').trim()
      if (ACCOUNT_KEY.test(key) && value) {
        pendingAccount = value
        flushPair()
        continue
      }
      if (PASSWORD_KEY.test(key) && value) {
        pendingPassword = value
        flushPair()
        continue
      }
      const parsed = parseSecretLine(value || key)
      if (parsed) {
        credentials.push(parsed)
        continue
      }
      if (key && value) {
        extraLines.push(`${key}: ${value}`)
      } else if (value || key) {
        extraLines.push(value || key)
      }
    }
    if (pendingAccount) {
      extraLines.push(pendingAccount)
    }
    if (pendingPassword) {
      extraLines.push(pendingPassword)
    }
  }

  if (credentials.length === 0) {
    const payload = String(fulfillment?.payload || '')
    if (!payload.trim()) {
      return { credentials, extraLines }
    }
    extraLines.length = 0
    for (const rawLine of payload.split('\n')) {
      const line = rawLine.trim()
      if (!line) {
        continue
      }
      const parsed = parseSecretLine(line)
      if (parsed) {
        credentials.push(parsed)
      } else {
        extraLines.push(line)
      }
    }
  }

  return { credentials, extraLines }
}
