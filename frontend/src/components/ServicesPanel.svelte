<script lang="ts">
  import { Database, Globe, KeyRound, Link2, Lock, Plus, RefreshCw, Search } from '@lucide/svelte'
  import { onMount } from 'svelte'
  import { describeError, type PorterApi } from '../lib/api'
  import type { App, CreateServiceResponse, Project, Service, ServiceTemplate } from '../lib/types'

  export let api: PorterApi
  export let projects: Project[] = []
  export let apps: App[] = []
  export let busy = false

  let loading = true
  let working = false
  let error = ''
  let search = ''
  let templates: ServiceTemplate[] = []
  let services: Service[] = []
  let selectedTemplateSlug = ''
  let projectID = ''
  let serviceName = ''
  let exposeService = false
  let attachServiceID = ''
  let attachAppID = ''
  let created: CreateServiceResponse | undefined
  let attachedEnv: Record<string, string> | undefined

  $: selectedTemplate = templates.find((template) => template.slug === selectedTemplateSlug)
  $: if (!selectedTemplateSlug && templates.length > 0) {
    selectedTemplateSlug = templates[0].slug
  }
  $: if (!projectID && projects.length > 0) {
    projectID = projects[0].id
  }
  $: if (!attachServiceID && services.length > 0) {
    attachServiceID = services[0].id
  }
  $: if (!attachAppID && apps.length > 0) {
    attachAppID = apps[0].id
  }
  $: if (selectedTemplate && !serviceName) {
    serviceName = selectedTemplate.slug
  }
  $: if (selectedTemplate) {
    exposeService = selectedTemplate.exposed || exposeService
  }

  onMount(loadServices)

  async function loadServices() {
    loading = true
    error = ''
    try {
      const [nextTemplates, nextServices] = await Promise.all([api.serviceTemplates(search.trim()), api.services()])
      templates = nextTemplates
      services = nextServices
      selectedTemplateSlug = nextTemplates.some((template) => template.slug === selectedTemplateSlug)
        ? selectedTemplateSlug
        : nextTemplates[0]?.slug ?? ''
      attachServiceID = nextServices.some((service) => service.id === attachServiceID)
        ? attachServiceID
        : nextServices[0]?.id ?? ''
    } catch (err) {
      error = describeError(err)
    } finally {
      loading = false
    }
  }

  async function createService() {
    if (!projectID || !selectedTemplateSlug || !serviceName.trim()) return
    working = true
    error = ''
    attachedEnv = undefined
    try {
      created = await api.createService({
        project_id: projectID,
        template_slug: selectedTemplateSlug,
        name: serviceName.trim(),
        exposed: exposeService,
      })
      serviceName = selectedTemplate?.slug ?? ''
      exposeService = selectedTemplate?.exposed ?? false
      await loadServices()
      attachServiceID = created.service.id
    } catch (err) {
      error = describeError(err)
    } finally {
      working = false
    }
  }

  async function attachService() {
    if (!attachServiceID || !attachAppID) return
    working = true
    error = ''
    try {
      const response = await api.attachService(attachServiceID, attachAppID)
      attachedEnv = response.env
    } catch (err) {
      error = describeError(err)
    } finally {
      working = false
    }
  }

  function templateProvides(template: ServiceTemplate) {
    return Object.keys(template.provides ?? {})
  }
</script>

<section class="services-panel" aria-labelledby="services-title">
  <div class="section-heading">
    <div>
      <h2 id="services-title">Services</h2>
      <p>{services.length} running service{services.length === 1 ? '' : 's'}</p>
    </div>
    <button class="icon-button" disabled={loading || working || busy} title="Refresh services" on:click={loadServices}>
      <RefreshCw size={15} />
    </button>
  </div>

  {#if error}
    <p class="form-error service-error">{error}</p>
  {/if}

  <div class="service-toolbar">
    <label>
      <span>Catalog</span>
      <div class="inline-control">
        <input bind:value={search} placeholder="postgres" />
        <button class="icon-button" disabled={loading || working} title="Search catalog" on:click={loadServices}>
          <Search size={15} />
        </button>
      </div>
    </label>
  </div>

  {#if loading}
    <div class="empty-list"><Database size={16} /> Loading catalog</div>
  {:else}
    <div class="catalog-grid">
      {#each templates as template (template.slug)}
        <button
          class:selected={template.slug === selectedTemplateSlug}
          class="service-card"
          type="button"
          on:click={() => {
            selectedTemplateSlug = template.slug
            serviceName = template.slug
            exposeService = template.exposed
          }}
        >
          <span class="service-card-head">
            <strong>{template.name}</strong>
            <small>{template.category || template.slug}</small>
          </span>
          <span>{template.image}</span>
          <span class="service-rules">
            <span>{template.exposed ? 'public' : 'private'} {#if template.exposed}<Globe size={13} />{:else}<Lock size={13} />{/if}</span>
            {#each templateProvides(template).slice(0, 2) as key}
              <span>{key}</span>
            {/each}
          </span>
        </button>
      {/each}
    </div>

    <form class="compact-form service-create-form" on:submit|preventDefault={createService}>
      <div class="form-grid">
        <label>
          <span>Project</span>
          <select bind:value={projectID} disabled={projects.length === 0}>
            {#each projects as project (project.id)}
              <option value={project.id}>{project.name}</option>
            {/each}
          </select>
        </label>
        <label>
          <span>Name</span>
          <input bind:value={serviceName} placeholder="db" />
        </label>
        <label class="check-row">
          <input bind:checked={exposeService} disabled={selectedTemplate?.exposed} type="checkbox" />
          <span>Public URL</span>
        </label>
      </div>
      <button class="primary-action" disabled={busy || working || !projectID || !selectedTemplateSlug || !serviceName.trim()} type="submit">
        <Plus size={16} />
        <span>Deploy service</span>
      </button>
    </form>

    {#if created}
      <div class="service-output">
        <div class="split-heading">
          <h3>Credentials</h3>
          <span><KeyRound size={14} /> shown once</span>
        </div>
        <div class="output-grid">
          {#each Object.entries(created.credentials) as [key, value]}
            <code>{key}={value}</code>
          {/each}
          {#each Object.entries(created.provides) as [key, value]}
            <code>{key}={value}</code>
          {/each}
        </div>
      </div>
    {/if}

    <div class="service-list">
      <div class="split-heading">
        <h3>Running</h3>
        <span>{services.length}</span>
      </div>
      {#if services.length === 0}
        <div class="empty-list"><Database size={16} /> No services</div>
      {:else}
        {#each services as service (service.id)}
          <div class="mini-row">
            <span>
              <strong>{service.name}</strong>
              <small>{service.template_slug} - {service.status}{service.hostname ? ` - ${service.hostname}` : ''}</small>
            </span>
            {#if service.exposed && service.hostname}
              <a class="icon-link" href={`https://${service.hostname}`} target="_blank" rel="noreferrer" title="Open service">
                <Globe size={15} />
              </a>
            {/if}
          </div>
        {/each}
      {/if}
    </div>

    <form class="compact-form service-attach-form" on:submit|preventDefault={attachService}>
      <div class="split-heading">
        <h3>Attach</h3>
        <Link2 size={15} />
      </div>
      <div class="form-grid two">
        <label>
          <span>Service</span>
          <select bind:value={attachServiceID} disabled={services.length === 0}>
            {#each services as service (service.id)}
              <option value={service.id}>{service.name}</option>
            {/each}
          </select>
        </label>
        <label>
          <span>App</span>
          <select bind:value={attachAppID} disabled={apps.length === 0}>
            {#each apps as app (app.id)}
              <option value={app.id}>{app.name}</option>
            {/each}
          </select>
        </label>
      </div>
      <button class="secondary-action" disabled={busy || working || !attachServiceID || !attachAppID} type="submit">
        <Link2 size={15} /> Attach
      </button>
      {#if attachedEnv}
        <div class="output-grid compact">
          {#each Object.entries(attachedEnv) as [key, value]}
            <code>{key}={value}</code>
          {/each}
        </div>
      {/if}
    </form>
  {/if}
</section>
