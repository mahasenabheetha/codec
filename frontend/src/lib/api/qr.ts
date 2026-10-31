// The QR code tool's jobs (internal/web/qr_api.go). Nothing is stored.
import { postRaw, request } from './client'

export type QRLevel = 'L' | 'M' | 'Q' | 'H'

export interface WiFi {
  ssid: string
  password: string
  /** WPA (also WPA2/WPA3), WEP, or none. */
  security: string
  hidden: boolean
}

export interface TOTP {
  issuer: string
  account: string
  /** Base32. */
  secret: string
  digits: number
  period: number
  algorithm: string
}

export type QRMake = { kind: 'text'; text: string } | { kind: 'wifi'; wifi: WiFi } | { kind: 'totp'; totp: TOTP }

export interface QRCode {
  /** The text encoded (the built WIFI: or otpauth:// for those kinds). */
  payload: string
  svg: string
  /** PNG, base64. */
  png: string
  size: number
  version: number
  level: QRLevel
}

export const makeQR = (m: QRMake, level: QRLevel) => request<QRCode>('POST', '/api/v2/qr/make', { ...m, level })

export interface QRMeaning {
  kind: 'url' | 'wifi' | 'otpauth' | 'email' | 'phone' | 'sms' | 'geo' | 'text'
  fields?: { name: string; value: string }[]
}

/** Read a code from a PNG, JPEG or GIF. */
export const readQR = (image: Blob) => postRaw<{ text: string; meaning: QRMeaning }>('/api/v2/qr/read', image)
