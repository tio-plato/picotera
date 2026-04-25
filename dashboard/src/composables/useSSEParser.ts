import { marked } from 'marked'
import DOMPurify from 'dompurify'

// ---- Types ----

export type SSEFormat = 'openai-chat' | 'anthropic' | 'openai-responses' | 'unknown'

export interface AggregatedResult {
  format: SSEFormat
  json: Record<string, unknown> | null
}

export interface ContentResult {
  thinking: string | null
  reply: string | null
}

interface SSEEvent {
  event?: string
  data: string
}

// ---- SSE Line Parsing ----

function parseSSEEvents(body: string): SSEEvent[] {
  const events: SSEEvent[] = []
  const chunks = body.split(/\n\n+/)
  for (const chunk of chunks) {
    const lines = chunk.split('\n')
    let eventType: string | undefined
    const dataParts: string[] = []
    for (const line of lines) {
      if (line.startsWith('event:')) {
        eventType = line.slice(6).trim()
      } else if (line.startsWith('data:')) {
        dataParts.push(line.slice(5).trimStart())
      }
    }
    if (dataParts.length === 0) continue
    const data = dataParts.join('\n')
    if (data === '[DONE]') continue
    events.push({ event: eventType, data })
  }
  return events
}

function parseEventData(event: SSEEvent): Record<string, unknown> | null {
  try {
    return JSON.parse(event.data) as Record<string, unknown>
  } catch {
    return null
  }
}

// ---- Format Detection ----

function detectFormat(events: SSEEvent[]): SSEFormat {
  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue

    const type = parsed.type as string | undefined
    if (typeof type === 'string') {
      if (type.startsWith('response.')) return 'openai-responses'
      if (
        type === 'message_start' ||
        type === 'message_delta' ||
        type === 'content_block_start' ||
        type === 'content_block_delta' ||
        type === 'content_block_stop' ||
        type === 'message_stop'
      ) {
        return 'anthropic'
      }
    }
    if (parsed.choices !== undefined) return 'openai-chat'
  }
  return 'unknown'
}

// ---- Aggregation: OpenAI Chat ----

function aggregateOpenAIChat(events: SSEEvent[]): Record<string, unknown> | null {
  let content = ''
  let role = ''
  let finishReason: string | null = null
  let id = ''
  let model = ''
  let usage: Record<string, unknown> | null = null

  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    if (parsed.id) id = parsed.id as string
    if (parsed.model) model = parsed.model as string
    const choices = parsed.choices as Array<Record<string, unknown>> | undefined
    if (choices?.[0]) {
      const choice = choices[0]
      const delta = choice.delta as Record<string, unknown> | undefined
      if (delta) {
        if (delta.content) content += delta.content as string
        if (delta.role) role = delta.role as string
      }
      if (choice.finish_reason) finishReason = choice.finish_reason as string
    }
    if (parsed.usage) usage = parsed.usage as Record<string, unknown>
  }

  return {
    id,
    object: 'chat.completion',
    model,
    choices: [{
      index: 0,
      message: { role: role || 'assistant', content },
      finish_reason: finishReason,
    }],
    ...(usage ? { usage } : {}),
  }
}

// ---- Aggregation: Anthropic ----

function aggregateAnthropic(events: SSEEvent[]): Record<string, unknown> | null {
  let id = ''
  let model = ''
  let role = 'assistant'
  let stopReason: string | null = null
  const contentBlocks: Array<{ type: string; text?: string; thinking?: string }> = []
  let currentBlockIndex = -1
  let usage: Record<string, unknown> | null = null

  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    const type = parsed.type as string

    if (type === 'message_start') {
      const msg = parsed.message as Record<string, unknown> | undefined
      if (msg) {
        id = (msg.id as string) || id
        model = (msg.model as string) || model
        role = (msg.role as string) || role
        if (msg.usage) usage = msg.usage as Record<string, unknown>
      }
    } else if (type === 'content_block_start') {
      const block = parsed.content_block as Record<string, unknown> | undefined
      if (block) {
        contentBlocks.push({ type: block.type as string })
        currentBlockIndex = contentBlocks.length - 1
      }
    } else if (type === 'content_block_delta') {
      const delta = parsed.delta as Record<string, unknown> | undefined
      if (delta && currentBlockIndex >= 0) {
        const block = contentBlocks[currentBlockIndex]
        if (delta.type === 'text_delta' && delta.text) {
          block.text = (block.text || '') + (delta.text as string)
        } else if (delta.type === 'thinking_delta' && delta.thinking) {
          block.thinking = (block.thinking || '') + (delta.thinking as string)
        }
      }
    } else if (type === 'message_delta') {
      const delta = parsed.delta as Record<string, unknown> | undefined
      if (delta?.stop_reason) stopReason = delta.stop_reason as string
      if (parsed.usage) {
        const u = parsed.usage as Record<string, unknown>
        usage = { ...usage, ...u }
      }
    }
  }

  return {
    id,
    type: 'message',
    role,
    content: contentBlocks,
    model,
    stop_reason: stopReason,
    ...(usage ? { usage } : {}),
  }
}

// ---- Aggregation: OpenAI Responses ----

function aggregateOpenAIResponses(events: SSEEvent[]): Record<string, unknown> | null {
  // Prefer response.completed event if present
  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    if (parsed.type === 'response.completed') {
      return parsed.response as Record<string, unknown> ?? null
    }
  }

  // Reconstruct from deltas
  const outputTexts: Map<string, string> = new Map()
  let id = ''
  let model = ''

  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue

    if (parsed.type === 'response.created' || parsed.type === 'response.in_progress') {
      const resp = parsed.response as Record<string, unknown> | undefined
      if (resp) {
        id = (resp.id as string) || id
        model = (resp.model as string) || model
      }
    } else if (parsed.type === 'response.output_text.delta') {
      const itemId = parsed.item_id as string
      const delta = parsed.delta as string
      outputTexts.set(itemId, (outputTexts.get(itemId) || '') + delta)
    }
  }

  const output: Array<Record<string, unknown>> = []
  for (const [itemId, text] of outputTexts) {
    output.push({
      id: itemId,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'output_text', text }],
    })
  }

  return {
    id,
    object: 'response',
    model,
    output,
  }
}

// ---- Content Extraction ----

function extractOpenAIChatContent(events: SSEEvent[]): ContentResult {
  let reply = ''
  let thinking = ''
  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    const choices = parsed.choices as Array<Record<string, unknown>> | undefined
    const delta = choices?.[0]?.delta as Record<string, unknown> | undefined
    if (!delta) continue
    if (delta.content) reply += delta.content as string
    if (delta.reasoning_content) thinking += delta.reasoning_content as string
  }
  return { thinking: thinking || null, reply: reply || null }
}

function extractAnthropicContent(events: SSEEvent[]): ContentResult {
  let thinking = ''
  let reply = ''

  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    const type = parsed.type as string

    if (type === 'content_block_delta') {
      const delta = parsed.delta as Record<string, unknown> | undefined
      if (delta?.type === 'text_delta' && delta.text) {
        reply += delta.text as string
      } else if (delta?.type === 'thinking_delta' && delta.thinking) {
        thinking += delta.thinking as string
      }
    }
  }
  return { thinking: thinking || null, reply: reply || null }
}

function extractOpenAIResponsesContent(events: SSEEvent[]): ContentResult {
  // Try response.completed first
  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    if (parsed.type === 'response.completed') {
      const resp = parsed.response as Record<string, unknown>
      const output = resp?.output as Array<Record<string, unknown>> | undefined
      let reply = ''
      let thinking = ''
      if (output) {
        for (const item of output) {
          const content = item.content as Array<Record<string, unknown>> | undefined
          if (content) {
            for (const part of content) {
              if (part.type === 'output_text' && part.text) reply += part.text as string
            }
          }
          const summary = item.summary as Array<Record<string, unknown>> | undefined
          if (summary) {
            for (const part of summary) {
              if (part.text) thinking += part.text as string
            }
          }
        }
      }
      return { thinking: thinking || null, reply: reply || null }
    }
  }

  // Fallback: concatenate deltas
  let reply = ''
  let thinking = ''
  for (const event of events) {
    const parsed = parseEventData(event)
    if (!parsed) continue
    if (parsed.type === 'response.output_text.delta' && parsed.delta) {
      reply += parsed.delta as string
    }
    if (parsed.type === 'response.reasoning_summary_text.delta' && parsed.delta) {
      thinking += parsed.delta as string
    }
  }
  return { thinking: thinking || null, reply: reply || null }
}

function extractJsonContent(body: string): ContentResult {
  try {
    const parsed = JSON.parse(body) as Record<string, unknown>

    // OpenAI Chat format
    const choices = parsed.choices as Array<Record<string, unknown>> | undefined
    if (choices?.[0]) {
      const msg = choices[0].message as Record<string, unknown> | undefined
      const reply = (msg?.content as string) || null
      const thinking = (msg?.reasoning_content as string) || null
      return { thinking, reply }
    }

    // Anthropic format
    if (parsed.type === 'message' && Array.isArray(parsed.content)) {
      let thinking = ''
      let reply = ''
      for (const block of parsed.content as Array<Record<string, unknown>>) {
        if (block.type === 'thinking' && block.thinking) thinking += block.thinking as string
        if (block.type === 'text' && block.text) reply += block.text as string
      }
      return { thinking: thinking || null, reply: reply || null }
    }

    // OpenAI Responses format
    if (parsed.object === 'response' && Array.isArray(parsed.output)) {
      let reply = ''
      let thinking = ''
      for (const item of parsed.output as Array<Record<string, unknown>>) {
        const content = item.content as Array<Record<string, unknown>> | undefined
        if (content) {
          for (const part of content) {
            if (part.type === 'output_text' && part.text) reply += part.text as string
          }
        }
        const summary = item.summary as Array<Record<string, unknown>> | undefined
        if (summary) {
          for (const part of summary) {
            if (part.text) thinking += part.text as string
          }
        }
      }
      return { thinking: thinking || null, reply: reply || null }
    }

    return { thinking: null, reply: null }
  } catch {
    return { thinking: null, reply: null }
  }
}

// ---- Public API ----

export function aggregateSSE(body: string): AggregatedResult {
  const events = parseSSEEvents(body)
  if (events.length === 0) {
    // Not SSE — try plain JSON
    try {
      return { format: 'unknown', json: JSON.parse(body) }
    } catch {
      return { format: 'unknown', json: null }
    }
  }

  const format = detectFormat(events)
  let json: Record<string, unknown> | null = null

  switch (format) {
    case 'openai-chat':
      json = aggregateOpenAIChat(events)
      break
    case 'anthropic':
      json = aggregateAnthropic(events)
      break
    case 'openai-responses':
      json = aggregateOpenAIResponses(events)
      break
    default:
      try {
        json = JSON.parse(body)
      } catch {
        json = null
      }
  }

  return { format, json }
}

export function extractContent(body: string, isSSE: boolean): ContentResult {
  if (!isSSE) return extractJsonContent(body)

  const events = parseSSEEvents(body)
  if (events.length === 0) return extractJsonContent(body)

  const format = detectFormat(events)

  switch (format) {
    case 'openai-chat':
      return extractOpenAIChatContent(events)
    case 'anthropic':
      return extractAnthropicContent(events)
    case 'openai-responses':
      return extractOpenAIResponsesContent(events)
    default:
      return { thinking: null, reply: null }
  }
}

export function isSSEContentType(headers: Record<string, string[]> | undefined): boolean {
  if (!headers) return false
  const ct = headers['Content-type'] ?? headers['content-type'] ?? headers['Content-Type'] ?? []
  const value = Array.isArray(ct) ? ct.join(', ') : ''
  return value.toLowerCase().includes('text/event-stream')
}

export function renderMarkdown(text: string): string {
  const html = marked.parse(text, { async: false }) as string
  return DOMPurify.sanitize(html)
}
