import { request } from './client'

// The Time jobs: POST /api/v2/time/{kind}. Nothing is stored.

const post = <T>(kind: string, body: object) => request<T>('POST', '/api/v2/time/' + kind, body)

export interface Stamp {
  kind: 'seconds' | 'milliseconds' | 'microseconds' | 'nanoseconds' | 'date'
  unix: number
  unixMs: number
  utc: string
  local: string
  readable: string
  http: string
  zone: string
  offset: string
  relative: string
  isoWeek: string
  dayOfYear: number
}

export interface CronField {
  name: string
  text: string
  values: number[]
  star: boolean
  meaning: string
}

export interface CronForm {
  kind: 'minutes' | 'hourly' | 'daily' | 'weekly' | 'monthly'
  every?: number
  minute: number
  time?: string
  days?: number[]
  day?: number
}

export interface CronResult {
  description: string
  standard?: string
  fields: CronField[]
  zone: string
  runs: string[] // RFC 3339, in the zone
  skipped: { wall: string; at: string }[]
  form?: CronForm
}

/** Body of a 422 for an expression of another dialect. */
export interface CronError {
  error: string
  dialect?: string
  field?: string
}

export const timestamp = (input: string, zone: string) => post<Stamp>('timestamp', { input, zone })
export const cron = (expr: string, zone: string, count = 10) => post<CronResult>('cron', { expr, zone, count })
export const buildCron = (form: CronForm) => post<{ expr: string }>('build', { form })
