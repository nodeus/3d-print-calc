<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type BackupManifest, type ImportReport } from '../lib/wails';
  import { t, locale, switchLocale } from '../lib/i18n';
  import { user, isAdmin } from '../lib/session';
  import { THEME_VARS, applyTheme, persistTheme, type ThemeVars } from '../lib/theme';
  import Users from './Users.svelte';

  let currency = $state('RUB');
  const codes = ['RUB', 'USD', 'EUR', 'KZT', 'BYN'];
  // Тема.
  let theme = $state('light');
  let customs: string[] = $state([]);
  let customName = $state('');
  let customVars = $state<ThemeVars>({});

  let backupPath = $state('');
  let backupManifest: BackupManifest | null = $state(null);
  let csvReport: ImportReport | null = $state(null);
  let csvApplied: ImportReport | null = $state(null);
  let importing = $state(false);
  let progress = $state({ done: 0, total: 0 });
  let error = $state('');
  // Нумерация заказов
  let numTemplate = $state('');
  let numSeq = $state('');
  let numPreview = $state('');
  // Колонки истории
  const allColumns = [
    'filename', 'start', 'end', 'status', 'duration', 'weight', 'length',
    'size', 'filament', 'slicer', 'cost', 'price', 'origin',
  ];
  let visibleColumns: string[] = $state([]);

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

  function toggleColumn(key: string) {
    visibleColumns = visibleColumns.includes(key)
      ? visibleColumns.filter((c) => c !== key)
      : [...visibleColumns, key];
  }

  async function saveColumns() {
    error = '';
    try {
      await wails.setHistoryColumns(visibleColumns);
    } catch (e) {
      error = String(e);
    }
  }

  // Свой пароль.
  let oldPw = $state('');
  let newPw = $state('');
  let pwMsg = $state('');

  async function changePw() {
    error = '';
    pwMsg = '';
    try {
      await wails.changeMyPassword(oldPw, newPw);
      oldPw = '';
      newPw = '';
      pwMsg = $t('set.pw_done');
    } catch (e) {
      error = String(e);
    }
  }

  function colLabel(key: string): string {
    const map: Record<string, string> = {
      filename: $t('hist.filename'),
      start: $t('hist.start'),
      end: $t('hist.end'),
      status: $t('hist.status'),
      duration: $t('hist.duration'),
      weight: $t('hist.weight'),
      length: $t('hist.length'),
      size: $t('hist.size'),
      filament: $t('hist.filament'),
      slicer: $t('hist.slicer'),
      cost: $t('hist.cost'),
      price: $t('calc.price'),
      origin: $t('hist.origin'),
    };
    return map[key] ?? key;
  }

  onMount(async () => {
    try {
      currency = await wails.getBaseCurrency();
      await loadNumbers();
      visibleColumns = await wails.historyColumns();
      const ui = await wails.uiSettings();
      theme = ui.theme || 'light';
      customs = Object.keys(ui.custom ?? {});
      // Стартовая точка редактора — первая кастомная (если есть).
      if (customs.length) customVars = { ...(ui.custom?.[customs[0]] ?? {}) };
    } catch (e) {
      console.error(e);
    }
  });

  async function pickTheme(name: string) {
    theme = name;
    error = '';
    try {
      const ui = await wails.uiSettings();
      applyTheme(name, ui.custom ?? {});
      await persistTheme(name);
    } catch (e) {
      error = String(e);
    }
  }

  async function saveCustom() {
    error = '';
    const name = customName.trim();
    if (!name) return;
    try {
      await wails.saveCustomTheme(name, customVars);
      const ui = await wails.uiSettings();
      customs = Object.keys(ui.custom ?? {});
      customName = '';
      await pickTheme(name);
    } catch (e) {
      error = String(e);
    }
  }

  async function removeCustom(name: string) {
    error = '';
    try {
      await wails.deleteCustomTheme(name);
      const ui = await wails.uiSettings();
      customs = Object.keys(ui.custom ?? {});
      if (theme === name) await pickTheme('light');
    } catch (e) {
      error = String(e);
    }
  }

  async function exportCustom(name: string) {
    error = '';
    try {
      await wails.exportThemeFile(name);
    } catch (e) {
      error = String(e);
    }
  }

  async function importCustom() {
    error = '';
    try {
      const name = await wails.importThemeFile();
      const ui = await wails.uiSettings();
      customs = Object.keys(ui.custom ?? {});
      if (name) await pickTheme(name);
    } catch (e) {
      error = String(e);
    }
  }

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

  async function applyCSV() {
    error = '';
    importing = true;
    progress = { done: 0, total: 0 };
    wails.onImportProgress((done, total) => {
      progress = { done, total };
    });
    try {
      csvApplied = await wails.applyHistoryCSV();
    } catch (e) {
      error = String(e);
    } finally {
      importing = false;
      wails.offImportProgress();
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
    {#if csvReport?.dryRun}<button class="warn" onclick={applyCSV} disabled={importing}>{$t('set.csv_apply')}</button>{/if}
  </div>
  {#if importing}<progress value={progress.done} max={progress.total || 1}></progress> {progress.done}/{progress.total}{/if}
  {#if csvReport}<p>{$t('set.csv_preview')}: total {csvReport.total}, new {csvReport.new}, existing {csvReport.existing}</p>{/if}
  {#if csvApplied}<p class="ok">{$t('set.csv_done')}: +{csvApplied.new} (stubs {csvApplied.stubs})</p>{/if}
</div>

{#if error}<p class="err">{error}</p>{/if}

<div class="card">
  <h3>{$t('set.theme')}</h3>
  <div class="row">
    <select value={theme} onchange={(e) => pickTheme(e.currentTarget.value)}>
      <option value="light">{$t('set.theme_light')}</option>
      <option value="dark">{$t('set.theme_dark')}</option>
      {#each customs as c}<option value={c}>{c}</option>{/each}
    </select>
  </div>
{#if canWrite($user)}
    <div class="row">
      <input placeholder={$t('set.theme_name')} bind:value={customName} />
      <button onclick={saveCustom}>{$t('set.theme_save')}</button>
      <button onclick={importCustom}>{$t('set.theme_import')}</button>
    </div>
    <div class="checks">
      {#each THEME_VARS as v}
        <label class="check"><input type="color" value={/^#[0-9a-fA-F]{6}$/.test(customVars[v] ?? '') ? customVars[v] : '#000000'} oninput={(e) => (customVars[v] = e.currentTarget.value)} /> {v}</label>
      {/each}
    </div>
    {#if customs.length}
      <div class="row">
        {#each customs as c}
          <span>{c} <button onclick={() => exportCustom(c)}>↓</button> <button onclick={() => removeCustom(c)}>✕</button></span>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<div class="card">
  <h3>{$t('set.pw')}</h3>
  <label>{$t('auth.password')} ({$t('set.pw_old')})<input type="password" bind:value={oldPw} /></label>
  <label>{$t('set.pw_new')}<input type="password" bind:value={newPw} /></label>
  <div class="row"><button onclick={changePw}>{$t('common.save')}</button></div>
  {#if pwMsg}<p class="ok">{pwMsg}</p>{/if}
</div>

<div class="card">
  <h3>{$t('set.columns')}</h3>
  <div class="checks">
    {#each allColumns as c}
      <label class="check"><input type="checkbox" checked={visibleColumns.includes(c)} onchange={() => toggleColumn(c)} /> {colLabel(c)}</label>
    {/each}
  </div>
  <div class="row"><button onclick={saveColumns}>{$t('common.save')}</button></div>
</div>

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
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; max-width: 560px; margin-bottom: 12px; }
  .card.wide { max-width: none; }
  h3 { margin: 0 0 8px; font-size: 15px; }
  label { display: flex; flex-direction: column; margin-bottom: 8px; font-size: 13px; }
  select { padding: 6px; font-size: 14px; }
  .note { color: var(--muted); font-size: 13px; }
  .row { display: flex; gap: 8px; flex-wrap: wrap; }
  .checks { display: flex; flex-wrap: wrap; gap: 4px 16px; margin-bottom: 8px; }
  label.check { display: inline-flex; align-items: center; gap: 6px; font-size: 14px; line-height: 1.4; white-space: nowrap; }
  label.check input { margin: 0; width: 16px; height: 16px; flex: none; }
  button { padding: 8px 14px; }
  button.warn { border-color: var(--err); color: var(--err); }
  .ok { color: green; word-break: break-all; }
  .err { color: var(--err); }
</style>
