import type { components } from '@/openapi-types'

export type ProviderView = components['schemas']['ProviderView']
export type ProviderModelEntry = components['schemas']['ProviderModelEntry']
export type CreateProviderRequestBody = components['schemas']['CreateProviderRequestBody']
export type ModelView = components['schemas']['ModelView']
export type EndpointView = components['schemas']['EndpointView']
export type ProviderEndpointView = components['schemas']['ProviderEndpointView']
export type RequestView = components['schemas']['RequestView']
export type RequestTraceView = components['schemas']['RequestTraceView']
export type TraceCostView = components['schemas']['TraceCostView']
export type ScriptView = components['schemas']['ScriptView']
export type ApiKeyView = components['schemas']['ApiKeyView']
export type ApiKeyMutateBody = components['schemas']['ApiKeyMutateBody']
export type ProjectView = components['schemas']['ProjectView']
export type UpsertProjectRequestBody = components['schemas']['UpsertProjectRequestBody']
export type FetchModelsRequestBody = components['schemas']['FetchModelsRequestBody']
export type FetchModelsResponseBody = components['schemas']['FetchModelsResponseBody']
export type ExchangeRateView = components['schemas']['ExchangeRateView']
export type Pricing = components['schemas']['Pricing']
export type PricingTier = components['schemas']['PricingTier']
export type PricingMatchCandidate = components['schemas']['PricingMatchCandidate']
export type OverviewSummaryView = components['schemas']['OverviewSummaryView']
export type OverviewDistributionView = components['schemas']['OverviewDistributionView']
export type OverviewDistributionRowView = components['schemas']['OverviewDistributionRowView']
export type OverviewSeriesView = components['schemas']['OverviewSeriesView']
export type OverviewSeriesGroupView = components['schemas']['OverviewSeriesGroupView']
export type OverviewSeriesPointView = components['schemas']['OverviewSeriesPointView']
export type OverviewWindowView = components['schemas']['OverviewWindowView']
export type OverviewCostView = components['schemas']['OverviewCostView']
export type OverviewBreakdownRowView = components['schemas']['OverviewBreakdownRowView']
export type KvEntryView = components['schemas']['KvEntryView']
export type KvMutateBody = components['schemas']['KvMutateBody']
export type SimulateDispatchRequestBody = components['schemas']['SimulateDispatchRequestBody']
export type SimulateDispatchResponseBody = components['schemas']['SimulateDispatchResponseBody']
export type SimulateCandidate = components['schemas']['SimulateCandidate']
export type SimulateLogEntry = components['schemas']['SimulateLogEntry']

export type OverviewRange = '1d' | '7d' | '1m'
export type OverviewDimension = 'apiKey' | 'model' | 'upstreamModel' | 'provider' | 'project'
export type OverviewSeriesDimension = 'none' | OverviewDimension
export type OverviewMetric = 'tokens' | 'cost' | 'requests' | 'traces'

export type EndpointType = NonNullable<EndpointView['endpointType']>
export const ENDPOINT_TYPES_MODEL_ROUTED: EndpointType[] = [
  'openaiChatCompletions',
  'openaiResponses',
  'anthropicMessages',
  'anthropicCountTokens',
  'geminiGenerateContent',
  'geminiStreamGenerateContent',
]
export const ENDPOINT_TYPE_LABELS: Record<EndpointType, string> = {
  general: '通用',
  openaiChatCompletions: 'OpenAI 聊天补全',
  openaiResponses: 'OpenAI 响应',
  anthropicMessages: 'Anthropic 消息',
  anthropicCountTokens: 'Anthropic Tokens 计数',
  geminiGenerateContent: 'Gemini 生成内容',
  geminiStreamGenerateContent: 'Gemini 流式生成内容',
  exaSearch: 'Exa 搜索',
  unknown: '未知',
}
