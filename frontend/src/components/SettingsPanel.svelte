<script lang="ts">
  import { KeyRound, Pencil, Plus, Trash2 } from '@lucide/svelte'
  import type { ApiToken, Project } from '../lib/types'

  export let projects: Project[] = []
  export let busy = false
  export let onCreateProject: (name: string) => Promise<Project>
  export let onUpdateProject: (id: string, name: string) => Promise<void>
  export let onDeleteProject: (id: string) => Promise<void>
  export let onCreateToken: (name: string, scopes: string[]) => Promise<ApiToken>

  let projectName = ''
  let tokenName = ''
  let token: ApiToken | undefined
  let tokenScopes = new Set(['apps:read', 'apps:deploy'])
  const scopes = ['projects:read', 'projects:write', 'apps:read', 'apps:write', 'apps:deploy', 'tokens:write']

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

  function toggleScope(scope: string) {
    if (tokenScopes.has(scope)) {
      tokenScopes.delete(scope)
    } else {
      tokenScopes.add(scope)
    }
    tokenScopes = new Set(tokenScopes)
  }
</script>

<section class="settings-panel" aria-labelledby="settings-title">
  <div class="section-heading">
    <div>
      <h2 id="settings-title">Settings</h2>
      <p>Projects and agent tokens</p>
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
