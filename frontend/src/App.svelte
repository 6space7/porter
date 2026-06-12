<script lang="ts">
  import {
    Activity,
    Boxes,
    LayoutDashboard,
    LogOut,
    Plus,
    RefreshCw,
    Server,
    Settings,
  } from '@lucide/svelte'
  import { onMount } from 'svelte'
  import AppDetail from './components/AppDetail.svelte'
  import AppTable from './components/AppTable.svelte'
  import CreateAppPanel from './components/CreateAppPanel.svelte'
  import LoginView from './components/LoginView.svelte'
  import ServicesPlaceholder from './components/ServicesPlaceholder.svelte'
  import SettingsPanel from './components/SettingsPanel.svelte'
  import { describeError, PorterApi } from './lib/api'
  import type { ApiToken, App, CreateAppInput, Project } from './lib/types'

  type View = 'apps' | 'create' | 'services' | 'settings'

  const api = new PorterApi()
  let authenticated = api.hasSessionHint()
  let booting = true
  let busy = false
  let error = ''
  let activeView: View = 'apps'
  let projects: Project[] = []
  let apps: App[] = []
  let selectedAppID = ''

  $: selectedApp = apps.find((app) => app.id === selectedAppID)
  $: runningCount = apps.filter((app) => app.status === 'running').length
  $: failedCount = apps.filter((app) => app.status === 'failed').length

  onMount(async () => {
    await loadWorkspace()
    booting = false
  })

  async function loadWorkspace() {
    error = ''
    try {
      const [nextProjects, nextApps] = await Promise.all([api.projects(), api.apps()])
      projects = nextProjects
      apps = nextApps
      authenticated = true
      if (!selectedAppID || !nextApps.some((app) => app.id === selectedAppID)) {
        selectedAppID = nextApps[0]?.id ?? ''
      }
    } catch (err) {
      authenticated = false
      error = ''
    }
  }

  async function doLogin(email: string, password: string) {
    busy = true
    error = ''
    try {
      await api.login(email, password)
      authenticated = true
      await loadWorkspace()
    } catch (err) {
      error = describeError(err)
    } finally {
      busy = false
    }
  }

  async function run(action: () => Promise<void>) {
    busy = true
    error = ''
    try {
      await action()
    } catch (err) {
      error = describeError(err)
    } finally {
      busy = false
    }
  }

  async function createProject(name: string) {
    let created: Project | undefined
    await run(async () => {
      created = await api.createProject(name)
      await loadWorkspace()
    })
    return created as Project
  }

  async function updateProject(id: string, name: string) {
    await run(async () => {
      await api.updateProject(id, name)
      await loadWorkspace()
    })
  }

  async function deleteProject(id: string) {
    await run(async () => {
      await api.deleteProject(id)
      await loadWorkspace()
    })
  }

  async function createApp(input: CreateAppInput) {
    await run(async () => {
      const app = await api.createApp(input)
      await loadWorkspace()
      selectedAppID = app.id
      activeView = 'apps'
    })
  }

  async function createToken(name: string, scopes: string[]) {
    let token: ApiToken | undefined
    await run(async () => {
      token = await api.createToken(name, scopes)
    })
    return token as ApiToken
  }

  async function logout() {
    await run(async () => {
      await api.logout()
      authenticated = false
      apps = []
      projects = []
      selectedAppID = ''
    })
  }

  function selectApp(appID: string) {
    selectedAppID = appID
    activeView = 'apps'
  }
</script>

{#if booting}
  <main class="boot-screen">
    <Server size={24} />
    <span>Loading porter</span>
  </main>
{:else if !authenticated}
  <LoginView busy={busy} error={error} onLogin={doLogin} />
{:else}
  <main class="app-shell">
    <aside class="sidebar" aria-label="Primary">
      <div class="brand-lockup">
        <span class="brand-mark"><Server size={20} /></span>
        <span>porter</span>
      </div>

      <nav>
        <button class:active={activeView === 'apps'} on:click={() => (activeView = 'apps')}>
          <LayoutDashboard size={17} /> Apps
        </button>
        <button class:active={activeView === 'create'} on:click={() => (activeView = 'create')}>
          <Plus size={17} /> New app
        </button>
        <button class:active={activeView === 'services'} on:click={() => (activeView = 'services')}>
          <Boxes size={17} /> Services
        </button>
        <button class:active={activeView === 'settings'} on:click={() => (activeView = 'settings')}>
          <Settings size={17} /> Settings
        </button>
      </nav>

      <div class="server-card">
        <span><Activity size={15} /> local server</span>
        <strong>{runningCount}/{apps.length}</strong>
      </div>
    </aside>

    <section class="workspace">
      <header class="topbar">
        <div>
          <h1>{activeView === 'apps' ? 'Deployment console' : activeView === 'create' ? 'Create app' : activeView === 'services' ? 'Services' : 'Settings'}</h1>
          <p>{failedCount > 0 ? `${failedCount} failed deployment${failedCount === 1 ? '' : 's'} need attention` : 'Local VPS - API first - HTTPS ready'}</p>
        </div>
        <div class="top-actions">
          <button class="secondary-action" disabled={busy} on:click={loadWorkspace}><RefreshCw size={15} /> Refresh</button>
          <button class="secondary-action" on:click={() => (activeView = 'create')}><Plus size={15} /> New app</button>
          <button class="icon-button" title="Log out" on:click={logout}><LogOut size={16} /></button>
        </div>
      </header>

      {#if error}
        <p class="global-error">{error}</p>
      {/if}

      <div class="workspace-grid">
        <section class="main-pane">
          {#if activeView === 'apps'}
            <AppTable {apps} {projects} {selectedAppID} onSelect={selectApp} />
          {:else if activeView === 'create'}
            <CreateAppPanel {projects} {busy} onCreateProject={createProject} onCreateApp={createApp} />
          {:else if activeView === 'services'}
            <ServicesPlaceholder />
          {:else}
            <SettingsPanel
              {projects}
              {busy}
              onCreateProject={createProject}
              onUpdateProject={updateProject}
              onDeleteProject={deleteProject}
              onCreateToken={createToken}
            />
          {/if}
        </section>

        <AppDetail
          {api}
          app={selectedApp}
          {busy}
          onChanged={loadWorkspace}
          onDeleted={() => {
            selectedAppID = ''
            void loadWorkspace()
          }}
        />
      </div>
    </section>
  </main>
{/if}
