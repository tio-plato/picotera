import createClient from 'openapi-fetch'
import type { paths, components } from '@/api'

const api = createClient<paths>({ baseUrl: '/' })

export type ProviderView = components['schemas']['ProviderView']
export type CreateProviderRequestBody = components['schemas']['CreateProviderRequestBody']
export type ModelView = components['schemas']['ModelView']
export type EndpointView = components['schemas']['EndpointView']
export type ModelProviderEndpointView = components['schemas']['ModelProviderEndpointView']
export type PaginatedBody = components['schemas']['PaginatedBodyModelProviderEndpointView']

export default api
