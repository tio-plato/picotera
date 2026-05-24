export type AggregatedFormat =
  | 'openaiChatCompletions'
  | 'openaiResponses'
  | 'anthropicMessages'
  | 'geminiStreamGenerateContent'

export interface AggregatedResponse {
  format: AggregatedFormat
  bodyEncoding: 'json'
  body?: unknown
  error?: string
}

export interface LogEntry {
  level: string
  message: string
  ts: string
}

export interface ArtifactPayload {
  method?: string
  url?: string
  statusCode?: number
  headers?: Record<string, string[]>
  body?: string
  bodyEncoding?: 'utf8' | 'base64'
  aggregated?: AggregatedResponse
  logs?: LogEntry[]
  timings?: number[]
}
