import type { components } from '@/openapi-types'

export type ProviderView = components['schemas']['ProviderView']
export type ProviderModelEntry = components['schemas']['ProviderModelEntry']
export type CreateProviderRequestBody = components['schemas']['CreateProviderRequestBody']
export type ModelView = components['schemas']['ModelView']
export type EndpointView = components['schemas']['EndpointView']
export type ProviderEndpointView = components['schemas']['ProviderEndpointView']
export type RequestView = components['schemas']['RequestView']
export type ScriptView = components['schemas']['ScriptView']
export type FetchModelsRequestBody = components['schemas']['FetchModelsRequestBody']
export type FetchModelsResponseBody = components['schemas']['FetchModelsResponseBody']

export type EndpointType = NonNullable<EndpointView['endpointType']>
export const ENDPOINT_TYPES_MODEL_ROUTED: EndpointType[] = [
  'openaiChatCompletions',
  'openaiResponses',
  'anthropicMessages',
  'anthropicCountTokens',
]
export const ENDPOINT_TYPES_DIRECT: EndpointType[] = ['general', 'generalListModels']
export const ENDPOINT_TYPE_LABELS: Record<EndpointType, string> = {
  general: '通用',
  openaiChatCompletions: 'OpenAI Chat Completions',
  openaiResponses: 'OpenAI Responses',
  anthropicMessages: 'Anthropic Messages',
  anthropicCountTokens: 'Anthropic Count Tokens',
  generalListModels: '通用 列表模型',
  unknown: '未知',
}
