<script lang="ts">
  import { ExternalLink, GitBranch, MoreHorizontal } from '@lucide/svelte'
  import type { App, Project } from '../lib/types'

  export let apps: App[] = []
  export let projects: Project[] = []
  export let selectedAppID = ''
  export let onSelect: (appID: string) => void

  function projectName(projectID: string) {
    return projects.find((project) => project.id === projectID)?.name ?? 'unknown'
  }
</script>

<section class="app-table-shell" aria-labelledby="apps-title">
  <div class="section-heading">
    <div>
      <h2 id="apps-title">Apps</h2>
      <p>{apps.length} tracked {apps.length === 1 ? 'app' : 'apps'}</p>
    </div>
  </div>

  <div class="app-table" role="table" aria-label="Apps">
    <div class="app-row table-head" role="row">
      <span role="columnheader">App</span>
      <span role="columnheader">Project</span>
      <span role="columnheader">Status</span>
      <span role="columnheader">Branch</span>
      <span role="columnheader">Port</span>
      <span role="columnheader">Open</span>
    </div>

    {#if apps.length === 0}
      <div class="empty-list">
        <MoreHorizontal size={20} />
        <span>No apps yet</span>
      </div>
    {:else}
      {#each apps as app (app.id)}
        <button
          class:selected={app.id === selectedAppID}
          class="app-row app-row-button"
          type="button"
          on:click={() => onSelect(app.id)}
        >
          <span class="app-name">
            <strong>{app.name}</strong>
            <small>{app.git_url.replace(/^https?:\/\//, '')}</small>
          </span>
          <span>{projectName(app.project_id)}</span>
          <span><i class:running={app.status === 'running'}></i>{app.status}</span>
          <span class="mono"><GitBranch size={14} /> {app.branch}</span>
          <span class="mono">{app.internal_port}</span>
          <span>
            <ExternalLink size={15} />
          </span>
        </button>
      {/each}
    {/if}
  </div>
</section>
