export type Project = {
  id: string
  name: string
}

export type App = {
  id: string
  project_id: string
  name: string
  git_url: string
  branch: string
  build_type: 'dockerfile' | 'nixpacks'
  internal_port: number
  status: string
}

export type Domain = {
  id: string
  app_id: string
  hostname: string
  type: 'generated' | 'custom'
  verified: boolean
}

export type EnvVar = {
  app_id: string
  key: string
  value: string
  is_secret: boolean
}

export type ServiceTemplate = {
  slug: string
  name: string
  description: string
  category: string
  docs_url: string
  logo: string
  image: string
  command?: string[]
  internal_port: number
  exposed: boolean
  variables: Record<string, string>
  provides: Record<string, string>
  volumes: Array<{ name: string; path: string }>
  healthcheck: { command: string }
}

export type Service = {
  id: string
  project_id: string
  server_id: string
  template_slug: string
  name: string
  status: string
  internal_port: number
  exposed: boolean
  hostname?: string
}

export type CreateServiceInput = {
  project_id: string
  template_slug: string
  name: string
  exposed: boolean
}

export type CreateServiceResponse = {
  service: Service
  credentials: Record<string, string>
  provides: Record<string, string>
}

export type AttachServiceResponse = {
  service_id: string
  app_id: string
  env: Record<string, string>
}

export type Deployment = {
  id: string
  app_id: string
  status: string
  stage: string
  build_log: string
  image_tag: string
}

export type CreateAppInput = {
  project_id: string
  name: string
  git_url: string
  branch: string
  build_type: 'dockerfile' | 'nixpacks'
  internal_port: number
}

export type UpdateAppInput = Partial<{
  name: string
  git_url: string
  branch: string
  build_type: 'dockerfile' | 'nixpacks'
  internal_port: number
}>

export type NewEnvVarInput = {
  key: string
  value: string
  is_secret: boolean
}

export type ApiToken = {
  token: string
  token_id: string
  name: string
  scopes: string[]
}

export type AgentDocs = {
  name: string
  description: string
  api_base: string
  mcp_endpoint: string
  auth: {
    header: string
    scheme: string
    example: string
  }
  tools: Array<{
    name: string
    scopes: string[]
    description: string
  }>
  examples: Array<{
    name: string
    description: string
    body: Record<string, unknown>
  }>
}

export type ApiErrorBody = {
  error: {
    code: string
    message: string
    hint: string
    details: Record<string, unknown>
  }
}

export type Workspace = {
  projects: Project[]
  apps: App[]
}
