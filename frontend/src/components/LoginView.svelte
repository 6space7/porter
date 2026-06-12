<script lang="ts">
  import { ArrowRight, LockKeyhole, Server } from '@lucide/svelte'

  export let busy = false
  export let error = ''
  export let onLogin: (email: string, password: string) => Promise<void>
  let email = 'admin@porter.local'
  let password = ''

  async function submit() {
    await onLogin(email, password)
  }
</script>

<main class="login-screen">
  <section class="login-panel" aria-labelledby="login-title">
    <div class="brand-lockup">
      <span class="brand-mark"><Server size={20} /></span>
      <span>porter</span>
    </div>

    <div class="login-copy">
      <h1 id="login-title">Deploy from your VPS</h1>
      <p>Sign in to manage apps, domains, deployments, and secrets.</p>
    </div>

    <form class="login-form" on:submit|preventDefault={submit}>
      <label>
        <span>Email</span>
        <input autocomplete="username" bind:value={email} inputmode="email" />
      </label>
      <label>
        <span>Password</span>
        <input autocomplete="current-password" bind:value={password} type="password" />
      </label>

      {#if error}
        <p class="form-error">{error}</p>
      {/if}

      <button class="primary-action" disabled={busy || !email || !password} type="submit">
        <LockKeyhole size={16} />
        <span>{busy ? 'Signing in' : 'Sign in'}</span>
        <ArrowRight size={16} />
      </button>
    </form>
  </section>
</main>
