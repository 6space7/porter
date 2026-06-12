import type {
  AgentDocs,
  ApiErrorBody,
  ApiToken,
  App,
  AttachServiceResponse,
  CreateAppInput,
  CreateServiceInput,
  CreateServiceResponse,
  Deployment,
  Domain,
  EnvVar,
  NewEnvVarInput,
  Project,
  Service,
  ServiceTemplate,
  UpdateAppInput,
} from './types'

type RequestBody = Record<string, unknown> | undefined

export class PorterApiError extends Error {
  code: string
  hint: string
  details: Record<string, unknown>
  status: number

  constructor(status: number, body: ApiErrorBody) {
    super(body.error.message)
    this.name = 'PorterApiError'
    this.status = status
    this.code = body.error.code
    this.hint = body.error.hint
    this.details = body.error.details
  }
}

export class PorterApi {
  private csrfToken = readCookie('porter_csrf')

  hasSessionHint() {
    return this.csrfToken !== ''
  }

  async login(email: string, password: string) {
    const response = await this.request<{ csrf_token: string }>('POST', '/auth/login', { email, password }, false)
    this.csrfToken = response.csrf_token || readCookie('porter_csrf')
  }

  async logout() {
    await this.request<void>('DELETE', '/auth/session')
    this.csrfToken = ''
  }

  async projects() {
    return this.request<Project[]>('GET', '/projects')
  }

  async createProject(name: string) {
    return this.request<Project>('POST', '/projects', { name })
  }

  async updateProject(id: string, name: string) {
    return this.request<Project>('PATCH', `/projects/${id}`, { name })
  }

  async deleteProject(id: string) {
    await this.request<void>('DELETE', `/projects/${id}`)
  }

  async apps() {
    return this.request<App[]>('GET', '/apps')
  }

  async createApp(input: CreateAppInput) {
    return this.request<App>('POST', '/apps', input as unknown as RequestBody)
  }

  async updateApp(id: string, input: UpdateAppInput) {
    return this.request<App>('PATCH', `/apps/${id}`, input as unknown as RequestBody)
  }

  async deleteApp(id: string) {
    await this.request<void>('DELETE', `/apps/${id}`)
  }

  async deployApp(id: string) {
    return this.request<Deployment>('POST', `/apps/${id}/deploy`)
  }

  async stopApp(id: string) {
    return this.request<App>('POST', `/apps/${id}/stop`)
  }

  async startApp(id: string) {
    return this.request<App>('POST', `/apps/${id}/start`)
  }

  async restartApp(id: string) {
    return this.request<App>('POST', `/apps/${id}/restart`)
  }

  async domains(appID: string) {
    return this.request<Domain[]>('GET', `/apps/${appID}/domains`)
  }

  async addDomain(appID: string, hostname: string) {
    return this.request<Domain>('POST', `/apps/${appID}/domains`, { hostname })
  }

  async verifyDomain(appID: string, domainID: string) {
    return this.request<Domain>('POST', `/apps/${appID}/domains/${domainID}/verify`)
  }

  async deleteDomain(appID: string, domainID: string) {
    await this.request<void>('DELETE', `/apps/${appID}/domains/${domainID}`)
  }

  async envVars(appID: string) {
    return this.request<EnvVar[]>('GET', `/apps/${appID}/env`)
  }

  async setEnvVar(appID: string, input: NewEnvVarInput) {
    return this.request<EnvVar>('POST', `/apps/${appID}/env`, input as unknown as RequestBody)
  }

  async deployments(appID: string) {
    return this.request<Deployment[]>('GET', `/apps/${appID}/deployments`)
  }

  async rollback(appID: string, deploymentID: string) {
    return this.request<Deployment>('POST', `/apps/${appID}/deployments/${deploymentID}/rollback`)
  }

  async buildLog(deploymentID: string) {
    return this.request<{ build_log: string }>('GET', `/deployments/${deploymentID}/build-log`)
  }

  async createToken(name: string, scopes: string[]) {
    return this.request<ApiToken>('POST', '/auth/tokens', { name, scopes })
  }

  async docs() {
    return this.request<AgentDocs>('GET', '/docs')
  }

  async serviceTemplates(search = '') {
    const query = search ? `?search=${encodeURIComponent(search)}` : ''
    return this.request<ServiceTemplate[]>('GET', `/service-templates${query}`)
  }

  async services() {
    return this.request<Service[]>('GET', '/services')
  }

  async createService(input: CreateServiceInput) {
    return this.request<CreateServiceResponse>('POST', '/services', input as unknown as RequestBody)
  }

  async attachService(serviceID: string, appID: string) {
    return this.request<AttachServiceResponse>('POST', `/services/${serviceID}/attach`, { app_id: appID })
  }

  runtimeLogURL(appID: string) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/api/v1/apps/${appID}/logs`
  }

  private async request<T>(method: string, path: string, body?: RequestBody, includeCSRF = true): Promise<T> {
    const headers: Record<string, string> = {}
    const unsafe = !['GET', 'HEAD', 'OPTIONS'].includes(method)
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json'
    }
    if (includeCSRF && unsafe) {
      const token = this.csrfToken || readCookie('porter_csrf')
      if (token) {
        headers['X-CSRF-Token'] = token
      }
    }

    const response = await fetch(`/api/v1${path}`, {
      method,
      credentials: 'same-origin',
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })

    if (response.status === 204) {
      return undefined as T
    }

    const text = await response.text()
    const payload = text ? JSON.parse(text) : undefined
    if (!response.ok) {
      throw new PorterApiError(response.status, payload as ApiErrorBody)
    }
    return payload as T
  }
}

export function describeError(error: unknown) {
  if (error instanceof PorterApiError) {
    return error.hint ? `${error.message} ${error.hint}` : error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return 'Something went wrong.'
}

function readCookie(name: string) {
  const match = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${name}=`))
  return match ? decodeURIComponent(match.slice(name.length + 1)) : ''
}
