export type ProviderEndpointFilters = Readonly<{ providerId?: number }>

export type RequestsFilters = Readonly<{
  type?: number
  providerId?: number
  endpointPath?: string
  model?: string
  upstreamModel?: string
  traceId?: string
}>

export type CursorFilters = Readonly<{ limit: number; cursor?: string }>
export type RequestListFilters = RequestsFilters & Partial<CursorFilters>

export const queryKeys = {
  providers: {
    all: ['providers'] as const,
    detail: (id: number) => ['providers', id] as const,
  },
  endpoints: {
    all: ['endpoints'] as const,
  },
  models: {
    all: ['models'] as const,
    detail: (name: string) => ['models', name] as const,
  },
  providerEndpoints: {
    all: ['providerEndpoints'] as const,
    list: (filters: ProviderEndpointFilters = {}) =>
      ['providerEndpoints', { ...filters }] as const,
  },
  scripts: {
    all: ['scripts'] as const,
    detail: (id: string) => ['scripts', id] as const,
  },
  apiKeys: {
    all: ['apiKeys'] as const,
    detail: (id: number) => ['apiKeys', id] as const,
  },
  exchangeRates: {
    all: ['exchangeRates'] as const,
    detail: (code: string) => ['exchangeRates', code] as const,
  },
  requests: {
    all: ['requests'] as const,
    list: (filters: RequestListFilters = {}) => ['requests', { ...filters }] as const,
    detail: (id: string) => ['requests', id] as const,
  },
  requestTraces: {
    all: ['requestTraces'] as const,
    list: (filters: CursorFilters) => ['requestTraces', { ...filters }] as const,
  },
  requestSpans: {
    all: ['requestSpans'] as const,
    detail: (requestId: string) => ['requestSpans', requestId] as const,
  },
  pricingMatches: {
    all: ['pricingMatches'] as const,
    model: (model: string) => ['pricingMatches', model] as const,
  },
  fetchModels: {
    all: ['fetchModels'] as const,
    source: (providerId: number, endpointPath: string) =>
      ['fetchModels', { providerId, endpointPath }] as const,
  },
  artifacts: {
    all: ['artifacts'] as const,
    detail: (url: string) => ['artifacts', url] as const,
  },
}
