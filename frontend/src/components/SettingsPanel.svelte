<script lang="ts">
  import { Bot, Check, Copy, ExternalLink, FileJson, KeyRound, Pencil, Plus, Trash2 } from '@lucide/svelte'
  import { onMount } from 'svelte'
  import type { PorterApi } from '../lib/api'
  import type { AgentDocs, ApiToken, Project } from '../lib/types'

  export let api: PorterApi
  export let projects: Project[] = []
  export let busy = false
  export let onCreateProject: (name: string) => Promise<Project>
  export let onUpdateProject: (id: string, name: string) => Promise<void>
  export let onDeleteProject: (id: string) => Promise<void>
  export let onCreateToken: (name: string, scopes: string[]) => Promise<ApiToken>

  let projectName = ''
  let tokenName = ''
  let token: ApiToken | undefined
  let agentToken: ApiToken | undefined
  let agentTokenName = 'agent'
  let agentDocs: AgentDocs | undefined
  let agentDocsError = ''
  let copied = ''
  let tokenScopes = new Set(['apps:read', 'apps:deploy'])
  const agentScopes = [
    'projects:read',
    'projects:write',
    'apps:read',
    'apps:write',
    'apps:deploy',
    'services:read',
    'services:write',
  ]
  const scopes = [...agentScopes, 'tokens:write']

  $: mcpEndpoint = absoluteURL(agentDocs?.mcp_endpoint ?? '/api/v1/mcp')
  $: llmsURL = absoluteURL('/llms.txt')
  $: docsURL = absoluteURL('/api/v1/docs')
  $: agentBearer = agentToken?.token ?? '<porter token>'
  $: claudeConfig = JSON.stringify(
    {
      mcpServers: {
        porter: {
          type: 'http',
          url: mcpEndpoint,
          headers: { Authorization: `Bearer ${agentBearer}` },
        },
      },
    },
    null,
    2,
  )
  $: cursorConfig = JSON.stringify(
    {
      mcpServers: {
        porter: {
          url: mcpEndpoint,
          headers: { Authorization: `Bearer ${agentBearer}` },
        },
      },
    },
    null,
    2,
  )

  onMount(() => {
    void loadAgentDocs()
  })

  async function createProject() {
    if (!projectName.trim()) return
    await onCreateProject(projectName.trim())
    projectName = ''
  }

  async function renameProject(project: Project) {
    const next = window.prompt('Project name', project.name)
    if (!next || next === project.name) return
    await onUpdateProject(project.id, next)
  }

  async function removeProject(project: Project) {
    if (!window.confirm(`Delete project ${project.name}? Apps in it will be removed too.`)) return
    await onDeleteProject(project.id)
  }

  async function createToken() {
    if (!tokenName.trim()) return
    token = await onCreateToken(tokenName.trim(), [...tokenScopes])
    tokenName = ''
  }

  async function createAgentToken() {
    agentToken = await onCreateToken(agentTokenName.trim() || 'agent', agentScopes)
  }

  function toggleScope(scope: string) {
    if (tokenScopes.has(scope)) {
      tokenScopes.delete(scope)
    } else {
      tokenScopes.add(scope)
    }
    tokenScopes = new Set(tokenScopes)
  }

  async function loadAgentDocs() {
    agentDocsError = ''
    try {
      agentDocs = await api.docs()
    } catch {
      agentDocsError = 'Agent docs unavailable'
    }
  }

  async function copyText(key: string, value: string) {
    await navigator.clipboard.writeText(value)
    copied = key
    window.setTimeout(() => {
      if (copied === key) copied = ''
    }, 1400)
  }

  function absoluteURL(value: string) {
    if (/^https?:\/\//.test(value)) return value
    const origin = typeof window === 'undefined' ? '' : window.location.origin
    return `${origin}${value}`
  }
</script>

<section class="settings-panel" aria-labelledby="settings-title">
  <div class="section-heading">
    <div>
      <h2 id="settings-title">Settings</h2>
      <p>Projects, tokens, and MCP</p>
    </div>
  </div>

  <div class="agent-onboarding" aria-labelledby="agent-title">
    <div class="agent-head">
      <span class="agent-icon"><Bot size={18} /></span>
      <div>
        <h3 id="agent-title">Connect your AI agent</h3>
        <p>{agentDocs ? `${agentDocs.tools.length} MCP tools available` : agentDocsError || 'MCP endpoint ready'}</p>
      </div>
      <div class="agent-links">
        <a href={llmsURL} target="_blank" rel="noreferrer"><FileJson size={14} /> llms.txt</a>
        <a href={docsURL} target="_blank" rel="noreferrer"><ExternalLink size={14} /> JSON docs</a>
      </div>
    </div>

    <label>
      <span>MCP endpoint</span>
      <div class="copy-field">
        <input readonly value={mcpEndpoint} />
        <button class="icon-button" title="Copy MCP endpoint" type="button" on:click={() => copyText('endpoint', mcpEndpoint)}>
          {#if copied === 'endpoint'}<Check size={15} />{:else}<Copy size={15} />{/if}
        </button>
      </div>
    </label>

    <div class="agent-token-row">
      <label>
        <span>Agent token name</span>
        <input bind:value={agentTokenName} placeholder="agent" />
      </label>
      <button class="primary-action" disabled={busy || !agentTokenName.trim()} type="button" on:click={createAgentToken}>
        <KeyRound size={15} /> Generate agent token
      </button>
    </div>

    {#if agentToken}
      <div class="token-output agent-token-output">
        <strong>Agent token shown once</strong>
        <div class="copy-field">
          <code>{agentToken.token}</code>
          <button class="icon-button" title="Copy agent token" type="button" on:click={() => copyText('agent-token', agentToken?.token ?? '')}>
            {#if copied === 'agent-token'}<Check size={15} />{:else}<Copy size={15} />{/if}
          </button>
        </div>
      </div>
    {/if}

    <div class="config-grid">
      <div class="config-block">
        <div class="split-heading">
          <span>Claude Code</span>
          <button class="icon-button" title="Copy Claude Code config" type="button" on:click={() => copyText('claude', claudeConfig)}>
            {#if copied === 'claude'}<Check size={15} />{:else}<Copy size={15} />{/if}
          </button>
        </div>
        <pre>{claudeConfig}</pre>
      </div>
      <div class="config-block">
        <div class="split-heading">
          <span>Cursor</span>
          <button class="icon-button" title="Copy Cursor config" type="button" on:click={() => copyText('cursor', cursorConfig)}>
            {#if copied === 'cursor'}<Check size={15} />{:else}<Copy size={15} />{/if}
          </button>
        </div>
        <pre>{cursorConfig}</pre>
      </div>
    </div>
  </div>

  <form class="compact-form project-form" on:submit|preventDefault={createProject}>
    <label>
      <span>Create project</span>
      <div class="inline-control">
        <input bind:value={projectName} placeholder="project-name" />
        <button class="icon-button" disabled={busy || !projectName.trim()} title="Create project" type="submit">
          <Plus size={16} />
        </button>
      </div>
    </label>
  </form>

  <div class="mini-list project-list">
    {#each projects as project (project.id)}
      <div class="mini-row">
        <span><strong>{project.name}</strong><small>{project.id}</small></span>
        <div class="row-actions">
          <button title="Rename project" on:click={() => renameProject(project)}><Pencil size={15} /></button>
          <button title="Delete project" on:click={() => removeProject(project)}><Trash2 size={15} /></button>
        </div>
      </div>
    {/each}
  </div>

  <form class="compact-form token-form" on:submit|preventDefault={createToken}>
    <label>
      <span>Token name</span>
      <input bind:value={tokenName} placeholder="agent-deploy" />
    </label>
    <div class="scope-grid">
      {#each scopes as scope}
        <label class="check-row">
          <input checked={tokenScopes.has(scope)} type="checkbox" on:change={() => toggleScope(scope)} />
          {scope}
        </label>
      {/each}
    </div>
    <button class="secondary-action" disabled={busy || !tokenName.trim() || tokenScopes.size === 0} type="submit">
      <KeyRound size={15} /> Create token
    </button>
  </form>

  {#if token}
    <div class="token-output">
      <strong>Token shown once</strong>
      <code>{token.token}</code>
    </div>
  {/if}
</section>
