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
