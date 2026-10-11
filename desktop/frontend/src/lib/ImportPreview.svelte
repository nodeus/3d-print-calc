<script lang="ts">
  import { t, locale } from './i18n';
  import { hms, num } from './format';
  import type { ImportReport } from './wails';

  let {
    report,
    perPage = 50,
    onApply,
    onClose,
  }: {
    report: ImportReport;
    perPage?: number;
    onApply: (selected: number[]) => void;
    onClose: () => void;
  } = $props();

  let page = $state(0);
  const rows = $derived(report.rows ?? []);
  // Отмеченные для импорта (индексы в rows); по умолчанию — все.
  // $effect: при новом отчёте выбор сбрасывается.
  let checked = $state<boolean[]>([]);
  $effect(() => {
    checked = rows.map(() => true);
  });
  const selectedCount = $derived(checked.filter(Boolean).length);
  const allChecked = $derived(rows.length > 0 && selectedCount === rows.length);
  function toggleAll() {
    const v = !allChecked;
    checked = rows.map(() => v);
  }
  function selected(): number[] {
    const out: number[] = [];
    for (let i = 0; i < checked.length; i++) if (checked[i]) out.push(i);
    return out;
  }
  const pp = $derived(perPage > 0 ? perPage : 50);
  const pages = $derived(Math.max(1, Math.ceil(rows.length / pp)));
  const safePage = $derived(Math.min(page, pages - 1));
  const visible = $derived(rows.slice(safePage * pp, safePage * pp + pp));
  const from = $derived(rows.length === 0 ? 0 : safePage * pp + 1);
  const to = $derived(Math.min(rows.length, safePage * pp + pp));

  function fmtDT(s: string): string {
    if (!s) return '—';
    const d = new Date(s);
    if (Number.isNaN(d.getTime())) return s;
    return d.toLocaleString($locale === 'en' ? 'en-US' : 'ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
</script>

<div class="iOverlay no-print" role="dialog" aria-modal="true" aria-label={$t('imp.title')}>
  <div class="iCard">
    <h3>{$t('imp.title')} ({report.total})</h3>
    <p class="note">{$t('set.csv_preview')}: total {report.total}, new {report.new}, existing {report.existing}</p>
    {#if rows.length === 0}
      <p class="note">{$t('imp.empty')}</p>
    {:else}
      <div class="twrap"><table class="tw">
        <thead><tr>
          <th><input type="checkbox" checked={allChecked} onchange={toggleAll} title={$t('imp.select_all')} aria-label={$t('imp.select_all')} /></th>
          <th>{$t('hist.filename')}</th>
          <th>{$t('imp.type')}</th>
          <th>{$t('hist.status')}</th>
          <th class="n">{$t('imp.size_b')}</th>
          <th>{$t('hist.start')}</th>
          <th>{$t('hist.end')}</th>
          <th class="n">{$t('imp.est_sec')}</th>
          <th class="n">{$t('hist.print_sec')}</th>
          <th class="n">{$t('hist.total_sec')}</th>
          <th class="n">{$t('imp.fil_total')}</th>
          <th class="n">{$t('imp.fil_used')}</th>
          <th>{$t('hist.slicer')}</th>
        </tr></thead>
        <tbody>
          {#each visible as r, vi}
            {@const gi = safePage * pp + vi}
            <tr class:dup={r.Exists} title={r.Exists ? $t('imp.dup') : ''}>
              <td><input type="checkbox" bind:checked={checked[gi]} aria-label={r.Filename} /></td>
              <td class="file" title={r.Filename}>{r.Filename}</td>
              <td>{r.Type || '—'}</td>
              <td>{r.Status || '—'}</td>
              <td class="n">{num(r.Size)}</td>
              <td>{fmtDT(r.StartTime)}</td>
              <td>{fmtDT(r.EndTime)}</td>
              <td class="n">{hms(Math.round(r.EstimatedTime))}</td>
              <td class="n">{hms(Math.round(r.PrintDuration))}</td>
              <td class="n">{hms(Math.round(r.TotalDuration))}</td>
              <td class="n">{num(Math.round(r.FilamentTotal * 100) / 100)}</td>
              <td class="n">{num(Math.round(r.FilamentUsed * 100) / 100)}</td>
              <td>{r.Slicer || '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table></div>
      <div class="row foot">
        <span class="note">{$t('hist.shown')} {from}–{to} / {rows.length}</span>
        <span class="pager">
          <button disabled={safePage === 0} onclick={() => page--} aria-label={$t('common.prev')}>‹</button>
          <button disabled={safePage >= pages - 1} onclick={() => page++} aria-label={$t('common.next')}>›</button>
        </span>
      </div>
    {/if}
    <div class="row">
      {#if report.new > 0}<button class="warn" onclick={() => onApply(selected())} disabled={selectedCount === 0}>{$t('set.csv_apply')} ({selectedCount})</button>{/if}
      <button onclick={onClose}>{$t('common.cancel')}</button>
    </div>
  </div>
</div>

<style>
  .iOverlay {
    position: fixed; inset: 0; background: rgba(0, 0, 0, 0.35);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }
  .iCard {
    background: var(--bg); border: 1px solid var(--card-border); border-radius: 8px;
    padding: 16px; max-width: min(1100px, 94vw); max-height: 86vh; overflow: auto;
  }
  .note { color: var(--muted); }
  tr.dup { background-color: color-mix(in srgb, var(--err) 8%, transparent); }
  td.file { max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .row { display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
  .foot { align-items: center; justify-content: space-between; }
  .pager { margin-left: auto; display: flex; gap: 4px; align-items: center; }
  table { font-size: 13px; }
</style>
