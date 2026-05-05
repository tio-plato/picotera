export type ArtifactHeaders = Record<string, string[]>

export interface ParsedJsonBody {
  ok: boolean
  value: unknown
  error: string
}

export function contentTypeHeaderValue(headers: ArtifactHeaders | undefined): string {
  if (!headers) return ''
  for (const [name, values] of Object.entries(headers)) {
    if (name.toLowerCase() === 'content-type') return values.join(', ')
  }
  return ''
}

export function isJsonContentType(headers: ArtifactHeaders | undefined): boolean {
  const value = contentTypeHeaderValue(headers)
  if (!value) return false
  const mediaType = value.split(';', 1)[0]?.toLowerCase() ?? ''
  return mediaType === 'application/json' || mediaType.endsWith('+json')
}

export function parseJsonBody(body: string | undefined, bodyEncoding: string | undefined): ParsedJsonBody {
  if (bodyEncoding === 'base64') {
    return { ok: false, value: null, error: '二进制内容不能解析为 JSON' }
  }
  if (body === undefined) {
    return { ok: false, value: null, error: '没有 body 内容' }
  }
  try {
    return { ok: true, value: JSON.parse(body), error: '' }
  } catch (e) {
    return {
      ok: false,
      value: null,
      error: e instanceof Error ? `JSON 解析失败: ${e.message}` : 'JSON 解析失败',
    }
  }
}

export function rawBodyText(body: string | undefined, bodyEncoding: string | undefined): string {
  if (bodyEncoding === 'base64') return ''
  return body ?? ''
}
