<script lang="ts">
  import { Database, Lock, Search } from '@lucide/svelte'
</script>

<section class="services-placeholder" aria-labelledby="services-title">
  <div class="section-heading">
    <div>
      <h2 id="services-title">Services</h2>
      <p>Catalog foundation for Phase 4</p>
    </div>
  </div>

  <div class="service-empty">
    <Database size={28} />
    <h3>Internal services land here next</h3>
    <p>Postgres, Redis, queues, and app attachments will use the same API-first model as apps.</p>
    <div class="service-rules">
      <span><Lock size={15} /> private by default</span>
      <span><Search size={15} /> searchable catalog</span>
    </div>
  </div>
</section>
