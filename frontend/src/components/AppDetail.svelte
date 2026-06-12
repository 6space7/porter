<script lang="ts">
  import {
    Activity,
    Boxes,
    ExternalLink,
    FileText,
    Globe2,
    Play,
    RefreshCw,
    RotateCcw,
    Save,
    Square,
    TerminalSquare,
    Trash2,
  } from '@lucide/svelte'
  import { onDestroy } from 'svelte'
  import type { PorterApi } from '../lib/api'
  import { describeError } from '../lib/api'
  import type { App, Deployment, Domain, EnvVar } from '../lib/types'

  export let api: PorterApi
  export let app: App | undefined
  export let busy = false
  export let onChanged: () => Promise<void>
  export let onDeleted: () => void

  let activeAppID = ''
  let activeAppEditKey = ''
  let error = ''
  let detailBusy = false
  let domains: Domain[] = []
  let envVars: EnvVar[] = []
  let deployments: Deployment[] = []
  let buildLog = ''
  let runtimeLog = ''
  let logState = 'idle'
  let socket: WebSocket | undefined

  let editName = ''
  let editBranch = ''
  let editGitURL = ''
  let editPort = 3000
  let domainHost = ''
  let envKey = ''
  let envValue = ''
  let envSecret = true

  $: if (app) {
    const appEditKey = [app.id, app.name, app.branch, app.git_url, app.internal_port].join('\u0000')
    const appChanged = app.id !== activeAppID
    if (appEditKey !== activeAppEditKey) {
      activeAppEditKey = appEditKey
      editName = app.name
      editBranch = app.branch
      editGitURL = app.git_url
      editPort = app.internal_port
    }
    if (appChanged) {
      activeAppID = app.id
      void loadDetail()
    }
  } else if (!app && activeAppID) {
    activeAppID = ''
    activeAppEditKey = ''
  }

  onDestroy(() => {
    socket?.close()
  })

  async function loadDetail() {
    if (!app) return
    detailBusy = true
    error = ''
    try {
      const [nextDomains, nextEnvVars, nextDeployments] = await Promise.all([
        api.domains(app.id),
        api.envVars(app.id),
        api.deployments(app.id),
      ])
      domains = nextDomains
      envVars = nextEnvVars
      deployments = nextDeployments
      buildLog = nextDeployments[0]?.build_log ?? ''
    } catch (err) {
      error = describeError(err)
    } finally {
      detailBusy = false
    }
  }

  async function run(action: () => Promise<void>) {
    error = ''
    detailBusy = true
    try {
      await action()
    } catch (err) {
      error = describeError(err)
    } finally {
      detailBusy = false
    }
  }

  async function saveSettings() {
    if (!app) return
    await run(async () => {
      await api.updateApp(app.id, {
        name: editName.trim(),
        branch: editBranch.trim(),
        git_url: editGitURL.trim(),
        internal_port: Number(editPort),
      })
      await onChanged()
      await loadDetail()
    })
  }

  async function lifecycle(action: 'start' | 'stop' | 'restart' | 'deploy') {
    if (!app) return
    await run(async () => {
      if (action === 'start') await api.startApp(app.id)
      if (action === 'stop') await api.stopApp(app.id)
      if (action === 'restart') await api.restartApp(app.id)
      if (action === 'deploy') await api.deployApp(app.id)
      await onChanged()
      await loadDetail()
    })
  }

  async function addDomain() {
    if (!app || !domainHost.trim()) return
    await run(async () => {
      await api.addDomain(app.id, domainHost.trim())
      domainHost = ''
      await loadDetail()
    })
  }

  async function verifyDomain(domainID: string) {
    if (!app) return
    await run(async () => {
      await api.verifyDomain(app.id, domainID)
      await loadDetail()
    })
  }

  async function deleteDomain(domainID: string) {
    if (!app || !window.confirm('Remove this domain from Porter?')) return
    await run(async () => {
      await api.deleteDomain(app.id, domainID)
      await loadDetail()
    })
  }

  async function setEnvVar() {
    if (!app || !envKey.trim()) return
    await run(async () => {
      await api.setEnvVar(app.id, {
        key: envKey.trim(),
        value: envValue,
        is_secret: envSecret,
      })
      envKey = ''
      envValue = ''
      envSecret = true
      await loadDetail()
    })
  }

  async function showBuildLog(deploymentID: string) {
    await run(async () => {
      const log = await api.buildLog(deploymentID)
      buildLog = log.build_log
    })
  }

  async function rollback(deploymentID: string) {
    if (!app || !window.confirm('Roll back to this deployment image?')) return
    await run(async () => {
      await api.rollback(app.id, deploymentID)
      await onChanged()
      await loadDetail()
    })
  }

  async function deleteApp() {
    if (!app || !window.confirm(`Delete ${app.name}?`)) return
    await run(async () => {
      await api.deleteApp(app.id)
      onDeleted()
    })
  }

  function toggleRuntimeLogs() {
    if (!app) return
    if (socket) {
      socket.close()
      socket = undefined
      logState = 'idle'
      return
    }
    runtimeLog = ''
    logState = 'connecting'
    socket = new WebSocket(api.runtimeLogURL(app.id))
    socket.onopen = () => (logState = 'streaming')
    socket.onmessage = (event) => (runtimeLog += String(event.data))
    socket.onerror = () => (logState = 'error')
    socket.onclose = () => {
      if (logState !== 'error') logState = 'idle'
      socket = undefined
    }
  }
</script>

{#if !app}
  <aside class="inspector empty-inspector">
    <Boxes size={24} />
    <h2>Select an app</h2>
    <p>Create or choose an app to manage domains, env vars, deploys, and logs.</p>
  </aside>
{:else}
  <aside class="inspector" aria-label="App detail">
    <div class="inspector-head">
      <div>
        <h2>{app.name}</h2>
        <p>{app.status} - {app.build_type} - port {app.internal_port}</p>
      </div>
      <span class="status-chip" class:running={app.status === 'running'}>{app.status}</span>
    </div>

    <div class="action-bar">
      <button title="Deploy" disabled={busy || detailBusy} on:click={() => lifecycle('deploy')}><Play size={16} /> Deploy</button>
      <button title="Stop" disabled={busy || detailBusy} on:click={() => lifecycle('stop')}><Square size={16} /> Stop</button>
      <button title="Start" disabled={busy || detailBusy} on:click={() => lifecycle('start')}><Play size={16} /> Start</button>
      <button title="Restart" disabled={busy || detailBusy} on:click={() => lifecycle('restart')}><RefreshCw size={16} /> Restart</button>
    </div>

    {#if error}
      <p class="form-error">{error}</p>
    {/if}

    <section class="inspector-section">
      <h3>Settings</h3>
      <form class="compact-form" on:submit|preventDefault={saveSettings}>
        <div class="form-grid two">
          <label><span>Name</span><input bind:value={editName} /></label>
          <label><span>Branch</span><input bind:value={editBranch} /></label>
        </div>
        <label><span>Git URL</span><input bind:value={editGitURL} /></label>
        <label><span>Internal port</span><input bind:value={editPort} min="1" max="65535" type="number" /></label>
        <button class="secondary-action" disabled={detailBusy} type="submit"><Save size={15} /> Save settings</button>
      </form>
    </section>

    <section class="inspector-section">
      <h3>Domains</h3>
      <form class="inline-control" on:submit|preventDefault={addDomain}>
        <input bind:value={domainHost} placeholder="app.example.com" />
        <button class="icon-button" disabled={detailBusy || !domainHost.trim()} title="Add domain" type="submit"><Globe2 size={16} /></button>
      </form>
      <div class="mini-list">
        {#each domains as domain (domain.id)}
          <div class="mini-row">
            <span>
              <strong>{domain.hostname}</strong>
              <small>{domain.type} - {domain.verified ? 'verified' : 'needs DNS'}</small>
            </span>
            <div class="row-actions">
              <a class="icon-link" href={`https://${domain.hostname}`} target="_blank" rel="noreferrer" title="Open domain"><ExternalLink size={15} /></a>
              <button title="Verify domain" on:click={() => verifyDomain(domain.id)}><RefreshCw size={15} /></button>
              <button title="Delete domain" on:click={() => deleteDomain(domain.id)}><Trash2 size={15} /></button>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <section class="inspector-section">
      <h3>Environment</h3>
      <form class="compact-form" on:submit|preventDefault={setEnvVar}>
        <div class="form-grid two">
          <label><span>Key</span><input bind:value={envKey} placeholder="DATABASE_URL" /></label>
          <label><span>Value</span><input bind:value={envValue} placeholder="postgres://..." /></label>
        </div>
        <label class="check-row"><input bind:checked={envSecret} type="checkbox" /> Secret</label>
        <button class="secondary-action" disabled={detailBusy || !envKey.trim()} type="submit"><Save size={15} /> Set variable</button>
      </form>
      <div class="mini-list">
        {#each envVars as env (env.key)}
          <div class="mini-row">
            <span><strong>{env.key}</strong><small>{env.is_secret ? 'secret' : env.value}</small></span>
          </div>
        {/each}
      </div>
    </section>

    <section class="inspector-section">
      <div class="split-heading">
        <h3>Deployments</h3>
        <button class="plain-button" on:click={loadDetail}><RefreshCw size={14} /> Refresh</button>
      </div>
      <div class="mini-list">
        {#each deployments as deployment (deployment.id)}
          <div class="mini-row">
            <span>
              <strong>{deployment.stage}</strong>
              <small>{deployment.id} - {deployment.status}</small>
            </span>
            <div class="row-actions">
              <button title="Build log" on:click={() => showBuildLog(deployment.id)}><FileText size={15} /></button>
              {#if deployment.status === 'running' && deployment.image_tag}
                <button title="Rollback" on:click={() => rollback(deployment.id)}><RotateCcw size={15} /></button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </section>

    <section class="inspector-section">
      <div class="split-heading">
        <h3>Logs</h3>
        <button class="plain-button" on:click={toggleRuntimeLogs}>
          <TerminalSquare size={14} /> {socket ? 'Stop stream' : 'Live logs'}
        </button>
      </div>
      <p class="log-state"><Activity size={13} /> {logState}</p>
      <pre>{runtimeLog || buildLog || 'No logs loaded yet.'}</pre>
    </section>

    <button class="danger-action" on:click={deleteApp}><Trash2 size={15} /> Delete app</button>
  </aside>
{/if}
