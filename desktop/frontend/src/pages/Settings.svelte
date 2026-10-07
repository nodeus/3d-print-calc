<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type BackupManifest, type ImportReport } from '../lib/wails';
  import { t, locale, switchLocale } from '../lib/i18n';
  import { user, isAdmin } from '../lib/session';
  import Users from './Users.svelte';

  let currency = $state('RUB');
  const codes = ['RUB', 'USD', 'EUR', 'KZT', 'BYN'];

  let backupPath = $state('');
  let backupManifest: BackupManifest | null = $state(null);
  let csvReport: ImportReport | null = $state(null);
  let csvApplied: ImportReport | null = $state(null);
  let error = $state('');
  // Нумерация заказов
  let numTemplate = $state('');
  let numSeq = $state('');
  let numPreview = $state('');

  async function loadNumbers() {
    try {
      const s = await wails.orderNumberSettings();
      numTemplate = s.template ?? '';
      numSeq = s.seq ?? '';
      numPreview = s.preview ?? '';
    } catch (e) {
      console.error(e);
    }
  }

  async function previewNumbers() {
    try {
      numPreview = await wails.previewOrderNumber(numTemplate);
    } catch (e) {
      numPreview = String(e);
    }
  }

  async function saveNumbers() {
    error = '';
    try {
      await wails.setOrderNumberSettings(numTemplate, numSeq);
      await loadNumbers();
    } catch (e) {
      error = String(e);
    }
  }

  onMount(async () => {
    try {
      currency = await wails.getBaseCurrency();
      await loadNumbers();
    } catch (e) {
      console.error(e);
    }
  });

  async function changeCurrency(code: string) {
    currency = code;
    try {
      await wails.setBaseCurrency(code);
    } catch (e) {
      console.error(e);
    }
  }

  async function run(fn: () => Promise<unknown>, done: (v: never) => void) {
    error = '';
    try {
      done((await fn()) as never);
    } catch (e) {
      error = String(e);
      console.error(e);
    }
  }
</script>

<div class="card">
  <label>{$t('set.language')}
    <select value={$locale} onchange={(e) => switchLocale(e.currentTarget.value)}>
      <option value="ru">Русский</option>
      <option value="en">English</option>
    </select>
  </label>
  <label>{$t('set.currency')}
    <select value={currency} onchange={(e) => changeCurrency(e.currentTarget.value)}>
      {#each codes as c}<option value={c}>{c}</option>{/each}
    </select>
  </label>
  <p class="note">{$t('set.rates_note')}</p>
</div>

<div class="card">
  <h3>{$t('set.backup')}</h3>
  <div class="row">
    <button onclick={() => run(wails.exportBackup, (p) => (backupPath = p))}>{$t('set.backup_save')}</button>
    <button onclick={() => run(wails.previewBackup, (m) => (backupManifest = m))}>{$t('set.backup_open')}</button>
    {#if backupManifest}<button class="warn" onclick={() => run(wails.applyBackup, (m) => (backupManifest = m))}>{$t('set.backup_apply')}</button>{/if}
  </div>
  {#if backupPath}<p class="ok">{backupPath}</p>{/if}
  {#if backupManifest}
    <p>v{backupManifest.version} · {backupManifest.created_at} · jobs: {backupManifest.counts?.jobs ?? '?'} · materials: {backupManifest.counts?.materials ?? '?'}</p>
  {/if}
</div>

<div class="card">
  <h3>{$t('set.csv')}</h3>
  <div class="row">
    <button onclick={() => run(wails.exportJobsCSV, (p) => (backupPath = p))}>{$t('set.csv_jobs_out')}</button>
    <button onclick={() => run(wails.exportMaterialsCSV, (p) => (backupPath = p))}>{$t('set.csv_mats_out')}</button>
    <button onclick={() => run(wails.previewHistoryCSV, (r) => (csvReport = r))}>{$t('set.csv_in')}</button>
    {#if csvReport?.dryRun}<button class="warn" onclick={() => run(wails.applyHistoryCSV, (r) => (csvApplied = r))}>{$t('set.csv_apply')}</button>{/if}
  </div>
  {#if csvReport}<p>{$t('set.csv_preview')}: total {csvReport.total}, new {csvReport.new}, existing {csvReport.existing}</p>{/if}
  {#if csvApplied}<p class="ok">{$t('set.csv_done')}: +{csvApplied.new} (stubs {csvApplied.stubs})</p>{/if}
</div>

{#if error}<p class="err">{error}</p>{/if}

<div class="card">
  <h3>{$t('set.ordnum')}</h3>
  <label>{$t('set.ordnum_tpl')}<input bind:value={numTemplate} oninput={previewNumbers} placeholder={'{YYYY}-{nnnn}'} /></label>
  <label>{$t('set.ordnum_seq')}<input type="number" min="1" bind:value={numSeq} oninput={previewNumbers} /></label>
  <p>{$t('set.ordnum_preview')}: <b>{numPreview}</b></p>
  <p class="note">{$t('set.ordnum_help')}</p>
  <div class="row"><button onclick={saveNumbers}>{$t('common.save')}</button></div>
</div>

{#if isAdmin($user)}
  <div class="card wide">
    <h3>{$t('nav.users')}</h3>
    <Users />
  </div>
{/if}

<style>
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; max-width: 560px; margin-bottom: 12px; }
  .card.wide { max-width: none; }
  h3 { margin: 0 0 8px; font-size: 15px; }
  label { display: flex; flex-direction: column; margin-bottom: 8px; font-size: 13px; }
  select { padding: 6px; font-size: 14px; }
  .note { color: #666; font-size: 13px; }
  .row { display: flex; gap: 8px; flex-wrap: wrap; }
  button { padding: 8px 14px; }
  button.warn { border-color: #b00; color: #b00; }
  .ok { color: green; word-break: break-all; }
  .err { color: #b00; }
</style>
