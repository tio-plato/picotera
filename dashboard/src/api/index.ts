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
export type ExchangeRateView = components['schemas']['ExchangeRateView']
export type Pricing = components['schemas']['Pricing']
export type PricingTier = components['schemas']['PricingTier']

export type EndpointType = NonNullable<EndpointView['endpointType']>
export const ENDPOINT_TYPES_MODEL_ROUTED: EndpointType[] = [
  'openaiChatCompletions',
  'openaiResponses',
  'anthropicMessages',
  'anthropicCountTokens',
  'geminiGenerateContent',
  'geminiStreamGenerateContent',
]
export const ENDPOINT_TYPES_DIRECT: EndpointType[] = ['general', 'generalListModels']
export const ENDPOINT_TYPE_LABELS: Record<EndpointType, string> = {
  general: '通用',
  openaiChatCompletions: 'OpenAI 聊天补全',
  openaiResponses: 'OpenAI 响应',
  anthropicMessages: 'Anthropic 消息',
  anthropicCountTokens: 'Anthropic Tokens 计数',
  generalListModels: '模型列表',
  geminiGenerateContent: 'Gemini 生成内容',
  geminiStreamGenerateContent: 'Gemini 流式生成内容',
  unknown: '未知',
}
