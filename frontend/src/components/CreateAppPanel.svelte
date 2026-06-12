<script lang="ts">
  import { GitPullRequest, Plus } from '@lucide/svelte'
  import type { CreateAppInput, Project } from '../lib/types'

  export let projects: Project[] = []
  export let busy = false
  export let onCreateProject: (name: string) => Promise<Project>
  export let onCreateApp: (input: CreateAppInput) => Promise<void>
  let projectName = ''
  let appName = ''
  let projectID = ''
  let gitURL = ''
  let branch = 'main'
  let buildType: 'dockerfile' | 'nixpacks' = 'dockerfile'
  let internalPort = 3000

  $: if (!projectID && projects.length > 0) {
    projectID = projects[0].id
  }

  $: if (projectID && projects.length > 0 && !projects.some((project) => project.id === projectID)) {
    projectID = projects[0].id
  }

  async function submitProject() {
    if (!projectName.trim()) return
    const project = await onCreateProject(projectName.trim())
    projectID = project.id
    projectName = ''
  }

  async function submitApp() {
    await onCreateApp({
      project_id: projectID,
      name: appName.trim(),
      git_url: gitURL.trim(),
      branch: branch.trim() || 'main',
      build_type: buildType,
      internal_port: Number(internalPort),
    })
    appName = ''
    gitURL = ''
    branch = 'main'
    internalPort = 3000
  }
</script>

<section class="create-panel" aria-labelledby="create-title">
  <div class="section-heading">
    <div>
      <h2 id="create-title">New app</h2>
      <p>Git repo to HTTPS app in one pass</p>
    </div>
  </div>

  <form class="compact-form project-form" on:submit|preventDefault={submitProject}>
    <label>
      <span>Project</span>
      <div class="inline-control">
        <input bind:value={projectName} placeholder="project-name" />
        <button class="icon-button" disabled={busy || !projectName.trim()} title="Create project" type="submit">
          <Plus size={16} />
        </button>
      </div>
    </label>
  </form>

  <form class="compact-form" on:submit|preventDefault={submitApp}>
    <label>
      <span>Use project</span>
      <select bind:value={projectID} disabled={projects.length === 0}>
        {#each projects as project (project.id)}
          <option value={project.id}>{project.name}</option>
        {/each}
      </select>
    </label>

    <label>
      <span>App name</span>
      <input bind:value={appName} placeholder="web" />
    </label>

    <label>
      <span>Git URL</span>
      <input bind:value={gitURL} placeholder="https://github.com/acme/app.git" />
    </label>

    <div class="form-grid">
      <label>
        <span>Branch</span>
        <input bind:value={branch} />
      </label>
      <label>
        <span>Build</span>
        <select bind:value={buildType}>
          <option value="dockerfile">Dockerfile</option>
          <option value="nixpacks">Nixpacks</option>
        </select>
      </label>
      <label>
        <span>Port</span>
        <input bind:value={internalPort} min="1" max="65535" type="number" />
      </label>
    </div>

    <button class="primary-action" disabled={busy || !projectID || !appName || !gitURL} type="submit">
      <GitPullRequest size={16} />
      <span>Create app</span>
    </button>
  </form>
</section>
