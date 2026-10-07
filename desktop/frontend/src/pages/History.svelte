<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type JobDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { num } from '../lib/format';

  let jobs: JobDTO[] = $state([]);
  onMount(async () => {
    try {
      jobs = await wails.jobs(200);
    } catch (e) {
      console.error(e);
    }
  });

  function costOf(j: JobDTO): string {
    try {
      const s = JSON.parse(j.CostSnapshot);
      return s.cost != null ? num(Number(s.cost)) : '—';
    } catch {
      return '—';
    }
  }
</script>

<table>
  <thead><tr><th>{$t('hist.filename')}</th><th>{$t('hist.start')}</th><th>{$t('hist.status')}</th><th>{$t('hist.weight')}</th><th>{$t('hist.cost')}</th></tr></thead>
  <tbody>
    {#each jobs as j}
      <tr><td class="file">{j.Filename}</td><td>{j.StartAt}</td><td>{j.Status}</td><td class="n">{num(j.WeightG)}</td><td class="n">{costOf(j)}</td></tr>
    {/each}
  </tbody>
</table>

<style>
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; font-variant-numeric: tabular-nums; }
  td.file { max-width: 420px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
