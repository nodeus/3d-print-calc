<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { wails, type BackupManifest, type ImportReport, type FxRate } from '../lib/wails';
  import { t, locale, switchLocale } from '../lib/i18n';
  import { setBaseSym } from '../lib/currency';
  import { user, isAdmin, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { THEME_PRESET_DARK, THEME_PRESET_LIGHT, applyTheme, persistTheme } from '../lib/theme';
  import { saveMySettingsPatch, mySettings } from '../lib/usersettings';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { calcFormDefaults, applyCalcDefaults, type CalcForm } from '../lib/calcform';
  import Users from './Users.svelte';

  let currency = $state('RUB');
  const codes = ['RUB', 'USD', 'EUR', 'KZT', 'BYN'];
  // Тема: базовые light/dark + 32 пресета. Выбранный пресет хранится
  // в БД как обычная кастомная тема (переживает перезапуск).
  let theme = $state('light');

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

  // Дефолты калькулятора (редактор): значения из файла настроек поверх заводских.
  const defKeys: { key: keyof CalcForm; label: string; text?: boolean }[] = [
    { key: 'matCode', label: 'calc.material', text: true },
    { key: 'timeH', label: 'calc.time_h' }, { key: 'timeM', label: 'calc.time_m' },
    { key: 'weightG', label: 'calc.weight' }, { key: 'filamentMM', label: 'calc.filament' },
    { key: 'tariff', label: 'calc.tariff' }, { key: 'powerKW', label: 'calc.power' },
    { key: 'amortPerHour', label: 'calc.amort' }, { key: 'maintPerHour', label: 'calc.maint' },
    { key: 'purgeG', label: 'calc.purge' },
    { key: 'opRate', label: 'calc.op_rate' }, { key: 'prepH', label: 'calc.prep' },
    { key: 'postH', label: 'calc.post' },
    { key: 'designerRate', label: 'calc.designer_rate' }, { key: 'modelH', label: 'calc.model_h' },
    { key: 'modelPrice', label: 'calc.model_price' },
    { key: 'extraMat', label: 'calc.extra' }, { key: 'fees', label: 'calc.fees' },
    { key: 'failPct', label: 'calc.fail' },
    { key: 'marginProfitPct', label: 'calc.margin_profit' },
    { key: 'marginDevPct', label: 'calc.margin_dev' },
    { key: 'copies', label: 'calc.copies' }, { key: 'vatPct', label: 'calc.vat' },
    { key: 'roundStep', label: 'calc.round' },
  ];
  let defVals: Record<string, string> = $state({});
  let defMsg = $state(false);

  function loadCalcDefaults() {
    const saved = (get(mySettings).calcDefaults ?? {}) as Record<string, unknown>;
    const merged = { ...calcFormDefaults, ...saved };
    defVals = {};
    for (const { key } of defKeys) {
      const v = (merged as Record<string, unknown>)[key];
      defVals[key] = v == null ? '' : String(v);
    }
  }

  async function saveCalcDefaults() {
    error = '';
    const out: Record<string, unknown> = {};
    for (const { key, text } of defKeys) {
      const raw = (defVals[key] ?? '').trim();
      if (text) {
        if (raw) out[key] = raw;
        continue;
      }
      if (raw === '') continue;
      const v = Number(raw.replace(',', '.'));
      if (Number.isFinite(v)) out[key] = v;
    }
    try {
      await saveMySettingsPatch({ calcDefaults: out });
      applyCalcDefaults(out);
      defMsg = true;
      setTimeout(() => (defMsg = false), 2500);
    } catch (e) {
      error = String(e);
    }
  }

  async function resetCalcDefaults() {
    error = '';
    try {
      await saveMySettingsPatch({ calcDefaults: {} });
      applyCalcDefaults({});
      loadCalcDefaults();
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

  onMount(async () => {
    try {
      currency = await wails.getBaseCurrency();
      setBaseSym(currency);
      await loadNumbers();
      await loadRates();
      await loadCalcDefaults();
      const ui = await wails.uiSettings();
      theme = ui.theme || 'light';
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
      await saveMySettingsPatch({ theme: name });
    } catch (e) {
      error = String(e);
    }
  }

  // pickPreset кладёт пресет в БД как кастомную тему и сразу применяет.
  async function pickPreset(name: string, vars: Record<string, string>) {
    error = '';
    try {
      await wails.saveCustomTheme(name, vars);
      await pickTheme(name);
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
      if (name) await pickTheme(name);
    } catch (e) {
      error = String(e);
    }
  }

  async function changeCurrency(code: string) {
    currency = code;
    setBaseSym(code);
    try {
      await wails.setBaseCurrency(code);
    } catch (e) {
      console.error(e);
    }
  }

  // Курсы и конвертация.
  let rates: FxRate[] = $state([]);
  let fxSortKey = $state('currency');
  let fxSortDir: 1 | -1 = $state(1);

  function toggleFxSort(key: string) {
    if (fxSortKey === key) fxSortDir = fxSortDir === 1 ? -1 : 1;
    else { fxSortKey = key; fxSortDir = 1; }
  }

  function fxArrow(key: string): string {
    return fxSortKey === key ? (fxSortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  let ratesShown = $derived([...rates].sort((a, b) => {
    const va = fxSortKey === 'rate' ? a.rate : fxSortKey === 'day' ? a.day : a.currency.toLowerCase();
    const vb = fxSortKey === 'rate' ? b.rate : fxSortKey === 'day' ? b.day : b.currency.toLowerCase();
    const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
    return cmp * fxSortDir;
  }));
  let rateCur = $state('');
  let rateVal = $state(0);
  let convCur = $state('');
  let convRate = $state(0);

  async function loadRates() {
    try {
      rates = (await wails.listFxRates()) ?? [];
    } catch (e) {
      error = String(e);
    }
  }

  async function saveRate() {
    error = '';
    if (!rateCur.trim() || !(rateVal > 0)) return;
    try {
      await wails.setFxRate(rateCur.trim().toUpperCase(), rateVal);
      rateCur = '';
      rateVal = 0;
      await loadRates();
    } catch (e) {
      error = String(e);
    }
  }

  async function removeRate(r: FxRate) {
    error = '';
    try {
      await wails.deleteFxRate(r.currency, r.day);
      await loadRates();
    } catch (e) {
      error = String(e);
    }
  }

  async function convertAll() {
    error = '';
    const target = convCur.trim().toUpperCase();
    if (!target || !(convRate > 0)) return;
    if (!(await askConfirm(`${$t('fx.convert_q')} ${target} @ ${convRate}? (${$t('fx.backup_warn')})`))) return;
    try {
      await wails.convertCurrency(target, convRate);
      currency = target;
      setBaseSym(target);
      convCur = '';
      convRate = 0;
      await loadRates();
    } catch (e) {
      error = String(e);
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

{#if error}<p class="err">{error}</p>{/if}

<div class="panels">
<div class="panel">
<div class="card">
  <label>{$t('set.language')}
    <select value={$locale} onchange={(e) => { switchLocale(e.currentTarget.value); saveMySettingsPatch({ locale: e.currentTarget.value }); }}>
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
  <h3>{$t('fx.title')}</h3>
  <table class="tw">
    <thead><tr>
      {#each [{ key: 'currency', label: 'fx.currency' }, { key: 'rate', label: 'fx.rate' }, { key: 'day', label: 'fx.day' }] as c}
        <th style={thWidth('fx', c.key)}>
          <button class="th" onclick={() => toggleFxSort(c.key)}>{$t(c.label)}{fxArrow(c.key)}</button>
          <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'fx', c.key)} ondblclick={() => resetColWidth('fx', c.key)}></span>
        </th>
      {/each}
      <th class="no-print"></th>
    </tr></thead>
    <tbody>
      {#each ratesShown as r}<tr><td>{r.currency}</td><td class="n">{r.rate}</td><td>{r.day}</td><td class="no-print"><button onclick={() => removeRate(r)}>✕</button></td></tr>{/each}
    </tbody>
  </table>
  {#if canWrite($user)}
    <div class="row">
      <input placeholder="USD" bind:value={rateCur} />
      <input type="number" min="0" step="any" placeholder={$t('fx.rate_ph')} bind:value={rateVal} />
      <button onclick={saveRate}>{$t('common.save')}</button>
    </div>
    <h3>{$t('fx.convert')}</h3>
    <p class="note">{$t('fx.convert_note')}</p>
    <div class="row">
      <input placeholder="USD" bind:value={convCur} />
      <input type="number" min="0" step="any" placeholder={$t('fx.rate_ph')} bind:value={convRate} />
      <button class="warn" onclick={convertAll}>{$t('fx.convert_go')}</button>
    </div>
  {/if}
</div>

<div class="card">
  <h3>{$t('set.ordnum')}</h3>
  <label>{$t('set.ordnum_tpl')}<input bind:value={numTemplate} oninput={previewNumbers} placeholder={'{YYYY}-{nnnn}'} /></label>
  <label>{$t('set.ordnum_seq')}<input placeholder="42" type="number" min="1" bind:value={numSeq} oninput={previewNumbers} /></label>
  <p>{$t('set.ordnum_preview')}: <b>{numPreview}</b></p>
  <p class="note">{$t('set.ordnum_help')}</p>
  <div class="row"><button onclick={saveNumbers}>{$t('common.save')}</button></div>
</div>

<div class="card">
  <h3>{$t('set.calcdef')}</h3>
  <p class="note">{$t('set.calcdef_note')}</p>
  <div class="defs">
    {#each defKeys as d}
      <label class="def">{$t(d.label)}
        {#if d.text}
          <input bind:value={defVals[d.key]} />
        {:else}
          <input type="number" step="any" bind:value={defVals[d.key]} />
        {/if}
      </label>
    {/each}
  </div>
  <div class="row">
    <button onclick={saveCalcDefaults}>{$t('common.save')}</button>
    <button onclick={resetCalcDefaults}>{$t('set.calcdef_reset')}</button>
  </div>
  {#if defMsg}<p class="ok">{$t('calc.saved')}</p>{/if}
</div>
</div>
<div class="panel">
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

</div>
<div class="panel">
<div class="card">
  <h3>{$t('set.theme')}</h3>
  <p class="note">{$t('set.theme_current')}: <b>{theme}</b></p>
  <h4>{$t('set.theme_base')}</h4>
  <div class="swatches">
    <button class="sw" class:on={theme === 'light'} onclick={() => pickTheme('light')} title={$t('set.theme_light')}>
      <span class="dots"><i style="background:#fafafa"></i><i style="background:#222222"></i><i style="background:#0645ad"></i><i style="background:#ffffff"></i></span>
      {$t('set.theme_light')}
    </button>
    <button class="sw" class:on={theme === 'dark'} onclick={() => pickTheme('dark')} title={$t('set.theme_dark')}>
      <span class="dots"><i style="background:#1a1d21"></i><i style="background:#e6e6e6"></i><i style="background:#7aa2ff"></i><i style="background:#2a2e34"></i></span>
      {$t('set.theme_dark')}
    </button>
  </div>
  <h4>{$t('set.theme_dark_presets')} ({THEME_PRESET_DARK.length})</h4>
  <div class="swatches">
    {#each THEME_PRESET_DARK as p}
      <button class="sw" class:on={theme === p.name} onclick={() => pickPreset(p.name, p.vars)} title={p.name}>
        <span class="dots"><i style="background:{p.vars.bg}"></i><i style="background:{p.vars.fg}"></i><i style="background:{p.vars.accent}"></i><i style="background:{p.vars['btn-bg']}"></i></span>
        {p.name}
      </button>
    {/each}
  </div>
  <h4>{$t('set.theme_light_presets')} ({THEME_PRESET_LIGHT.length})</h4>
  <div class="swatches">
    {#each THEME_PRESET_LIGHT as p}
      <button class="sw" class:on={theme === p.name} onclick={() => pickPreset(p.name, p.vars)} title={p.name}>
        <span class="dots"><i style="background:{p.vars.bg}"></i><i style="background:{p.vars.fg}"></i><i style="background:{p.vars.accent}"></i><i style="background:{p.vars['btn-bg']}"></i></span>
        {p.name}
      </button>
    {/each}
  </div>
{#if canWrite($user)}
  <div class="row">
    <button onclick={() => exportCustom(theme)}>{$t('set.theme_save')}</button>
    <button onclick={importCustom}>{$t('set.theme_import')}</button>
  </div>
{/if}
</div>

</div>
<div class="panel">
<div class="card">
  <h3>{$t('set.pw')}</h3>
  <label>{$t('auth.password')} ({$t('set.pw_old')})<input type="password" bind:value={oldPw} /></label>
  <label>{$t('set.pw_new')}<input type="password" bind:value={newPw} /></label>
  <div class="row"><button onclick={changePw}>{$t('common.save')}</button></div>
  {#if pwMsg}<p class="ok">{pwMsg}</p>{/if}
</div>

{#if isAdmin($user)}
  <div class="card wide">
    <h3>{$t('nav.users')}</h3>
    <Users />
  </div>
{/if}
</div>
</div>

<style>
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; margin-bottom: 12px; }
  .panels { display: grid; grid-template-columns: 1fr 1fr; gap: 0 12px; align-items: start; }
  .defs { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0 8px; }
  label.def input { width: 100%; box-sizing: border-box; }
  .card.wide { max-width: none; }
  h3 { margin: 0 0 8px; font-size: 15px; }
  h4 { margin: 12px 0 6px; font-size: 13px; color: var(--muted); }
  .swatches { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 4px; }
  button.sw { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; padding: 8px 10px; font-size: 12px; min-width: 118px; }
  button.sw.on { outline: 2px solid var(--accent); }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 14px; }
  .dots { display: flex; gap: 3px; }
  .dots i { width: 16px; height: 16px; border-radius: 50%; border: 1px solid var(--sw-border); }
  label { display: flex; flex-direction: column; margin-bottom: 8px; font-size: 13px; }
  select { padding: 6px; font-size: 14px; }
  .note { color: var(--muted); font-size: 13px; }
  .row { display: flex; gap: 8px; flex-wrap: wrap; }
  button { padding: 8px 14px; }
  button.warn { border-color: var(--err); color: var(--err); }
  .ok { color: green; word-break: break-all; }
  .err { color: var(--err); }
</style>
