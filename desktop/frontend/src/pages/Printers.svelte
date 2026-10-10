<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type PrinterDTO, type Printer, type ImportReport, type PartDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { notify } from '../lib/notice';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money, num } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';
  import { colKeyMove } from '../lib/colkeyboard';

  let printers: PrinterDTO[] = $state([]);
  let error = $state('');
  let editId = $state(0);
  let sortKey = $state('name');
  let sortDir: 1 | -1 = $state(1);

  function toggleSort(key: string) {
    if (sortKey === key) sortDir = sortDir === 1 ? -1 : 1;
    else { sortKey = key; sortDir = 1; }
  }

  function arrow(key: string): string {
    return sortKey === key ? (sortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  function sortVal(p: PrinterDTO): string | number {
    switch (sortKey) {
      case 'name': return p.Name.toLowerCase();
      case 'model': return (p.Model || '').toLowerCase();
      case 'price': return p.PurchasePrice;
      case 'amort': return p.amortPerHour;
      case 'method': return p.AmortMethod || '';
      default: return p.Name.toLowerCase();
    }
  }

  let shown = $derived([...printers].sort((a, b) => {
    const va = sortVal(a);
    const vb = sortVal(b);
    const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
    return cmp * sortDir;
  }));

  const printerCols = [
    { key: 'name', label: 'printer.name' }, { key: 'model', label: 'printer.model' },
    { key: 'price', label: 'printer.price', num: true }, { key: 'amort', label: 'printer.amort', num: true },
    { key: 'method', label: 'printer.method' },
  ];
  const printerKeys = printerCols.map((c) => c.key);
  const printerNum = (k: string) => printerCols.some((c) => c.key === k && c.num);
  const printerLabel = (k: string) => printerCols.find((c) => c.key === k)?.label ?? k;

  let printerOrd = $derived(orderFor($mySettings.colOrder, 'printers', printerKeys));

  function printerText(p: PrinterDTO, k: string): string {
    switch (k) {
      case 'name': return p.Name;
      case 'model': return p.Model;
      case 'price': return money(p.PurchasePrice / 100, p.Currency, 'ru');
      case 'amort': return num(p.amortPerHour);
      case 'method': return p.AmortMethod;
      default: return '';
    }
  }

  let dragKey: string | null = $state(null);
  let dropX: number | null = $state(null);
  let wrapEl: HTMLDivElement | null = $state(null);

  async function dropCols(next: string[]) {
    error = '';
    try {
      await saveTableOrder('printers', next);
    } catch (e) {
      error = String(e);
    }
  }
  // Форма: числа nullable (пусто — виден плейсхолдер, в payload — ?? дефолт).
  type PrinterForm = Omit<Printer, 'PurchasePrice' | 'LifeYears' | 'HoursPerDay' | 'PowerKW' | 'MaintPerHour' | 'ResourceHours'> & {
    PurchasePrice: number | null; LifeYears: number | null; HoursPerDay: number | null;
    PowerKW: number | null; MaintPerHour: number | null; ResourceHours: number | null;
  };
  let form: PrinterForm = $state(blank());
  let isNew = $state(false);

  // Moonraker
  let urls: Record<number, string> = $state({});
  // Закупки запчастей/ТО по принтерам.
  let parts: Record<number, PartDTO[]> = $state({});
  let partEdit: { id: number; printerId: number; boughtAt: string; name: string; price: number | null; qty: number | null; note: string } | null = $state(null);

  function newPart(printerId: number) {
    partEdit = { id: 0, printerId, boughtAt: '', name: '', price: null, qty: null, note: '' };
  }

  function editPart(p: PartDTO) {
    partEdit = { id: p.id, printerId: p.printerId, boughtAt: p.boughtAt, name: p.name, price: p.price, qty: p.qty, note: p.note };
  }

  async function loadParts(printerId: number) {
    try {
      parts[printerId] = (await wails.getParts(printerId)) ?? [];
    } catch (e) {
      error = String(e);
    }
  }

  async function savePart() {
    if (!partEdit || !partEdit.name.trim()) return;
    error = '';
    try {
      await wails.savePart({
        id: partEdit.id, printerId: partEdit.printerId, boughtAt: partEdit.boughtAt,
        name: partEdit.name.trim(), price: partEdit.price ?? 0, qty: partEdit.qty ?? 1,
        total: 0, note: partEdit.note.trim(),
      });
      const pid = partEdit.printerId;
      partEdit = null;
      await loadParts(pid);
      notify($t('common.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function removePart(p: PartDTO) {
    error = '';
    if (!(await askConfirm(`${$t('common.delete_q')} «${p.name}»?`))) return;
    try {
      await wails.deletePart(p.id);
      await loadParts(p.printerId);
    } catch (e) {
      error = String(e);
    }
  }
  let status: Record<number, string> = $state({});
  // Импорт Moonraker — состояние на принтер (карточки не затирают друг друга).
  let report: Record<number, ImportReport | null> = $state({});
  let applied: Record<number, ImportReport | null> = $state({});
  let importing: Record<number, boolean> = $state({});
  let progress: Record<number, { done: number; total: number }> = $state({});
  // Лимит последних работ Moonraker (null — пусто, т.е. вся история).
  let moonLimit: Record<number, number | null> = $state({});

  function blank(): PrinterForm {
    return {
      ID: 0, Name: '', Model: '', PurchasePrice: null, Currency: 'RUB',
      CommissioningDate: '', LifeYears: null, HoursPerDay: null,
      CalendarMode: 'calendar', AmortMethod: 'calendar', PowerKW: null,
      MaintPerHour: null, ResourceHours: null, MoonrakerURL: '',
    };
  }

  // Сортировка запчастей (общая на все карточки; ширин/drag у малой таблицы нет).
  let partSortKey = $state('date');
  let partSortDir: 1 | -1 = $state(-1);

  function togglePartSort(key: string) {
    if (partSortKey === key) partSortDir = partSortDir === 1 ? -1 : 1;
    else { partSortKey = key; partSortDir = 1; }
  }

  function partArrow(key: string): string {
    return partSortKey === key ? (partSortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  function partVal(pt: PartDTO, key: string): string | number {
    switch (key) {
      case 'date': return pt.boughtAt || '';
      case 'name': return (pt.name || '').toLowerCase();
      case 'price': return pt.price;
      case 'qty': return pt.qty;
      case 'total': return pt.total;
      case 'note': return (pt.note || '').toLowerCase();
      default: return pt.boughtAt || '';
    }
  }

  function sortedParts(pid: number): PartDTO[] {
    const dir = partSortDir;
    return [...(parts[pid] ?? [])].sort((a, b) => {
      const va = partVal(a, partSortKey);
      const vb = partVal(b, partSortKey);
      const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
      return cmp * dir;
    });
  }

  async function load() {
    try {
      printers = (await wails.printers()) ?? [];
      for (const p of printers) urls[p.ID] = p.MoonrakerURL ?? '';
      for (const p of printers) await loadParts(p.ID);
    } catch (e) {
      error = String(e);
    }
  }
  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
  });

  function startAdd() {
    form = blank();
    isNew = true;
    editId = 0;
  }

  function startEdit(p: PrinterDTO) {
    form = {
      ID: p.ID, Name: p.Name, Model: p.Model, PurchasePrice: p.PurchasePrice / 100, Currency: p.Currency,
      CommissioningDate: p.CommissioningDate, LifeYears: p.LifeYears, HoursPerDay: p.HoursPerDay,
      CalendarMode: p.CalendarMode, AmortMethod: p.AmortMethod, PowerKW: p.PowerKW,
      MaintPerHour: p.MaintPerHour / 100, ResourceHours: p.ResourceHours, MoonrakerURL: p.MoonrakerURL ?? '',
    };
    isNew = false;
    editId = p.ID;
  }

  async function save() {
    error = '';
    try {
      const payload: Printer = {
        ...form,
        PurchasePrice: Math.round((form.PurchasePrice ?? 0) * 100),
        LifeYears: form.LifeYears ?? 3,
        HoursPerDay: form.HoursPerDay ?? 8,
        PowerKW: form.PowerKW ?? 0.35,
        MaintPerHour: Math.round((form.MaintPerHour ?? 0) * 100),
        ResourceHours: form.ResourceHours ?? 0,
      };
      if (isNew) await wails.addPrinter(payload);
      else await wails.updatePrinter(payload);
      editId = 0;
      isNew = false;
      await load();
      notify($t('printer.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(p: PrinterDTO) {
    error = '';
    if (!(await askConfirm(`${$t("common.delete_q")} «${p.Name}»?`))) return;
    try {
      await wails.deletePrinter(p.ID);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function saveURL(p: PrinterDTO) {
    error = '';
    try {
      await wails.setMoonrakerURL(p.ID, (urls[p.ID] ?? '').trim());
      await load();
      notify($t('common.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function ping(p: PrinterDTO) {
    error = '';
    try {
      status[p.ID] = await wails.moonrakerStatus(p.ID);
    } catch (e) {
      error = String(e);
      status[p.ID] = '';
    }
  }

  async function preview(p: PrinterDTO) {
    error = '';
    applied[p.ID] = null;
    try {
      report[p.ID] = await wails.moonrakerPreview(p.ID, moonLimit[p.ID] ?? 0);
    } catch (e) {
      error = String(e);
    }
  }

  async function apply(p: PrinterDTO) {
    error = '';
    importing[p.ID] = true;
    progress[p.ID] = { done: 0, total: 0 };
    wails.onImportProgress((done, total) => {
      progress[p.ID] = { done, total };
    });
    try {
      applied[p.ID] = await wails.moonrakerApply();
      report[p.ID] = null;
    } catch (e) {
      error = String(e);
    } finally {
      importing[p.ID] = false;
      wails.offImportProgress();
    }
  }

  async function toPDF() {
    const sections = [
      {
        Title: '',
        Headers: printerOrd.map((k) => $t(printerLabel(k))),
        Rows: shown.map((p) => printerOrd.map((k) => printerText(p, k))),
      },
    ];
    for (const p of shown) {
      const list = parts[p.ID] ?? [];
      if (!list.length) continue;
      sections.push({
        Title: `${p.Name} — ${$t('part.title')}`,
        Headers: [$t('part.date'), $t('part.name'), $t('part.price'), $t('part.qty'), $t('part.total'), $t('part.note')],
        Rows: list.map((pt) => [pt.boughtAt || '—', pt.name, money(pt.price, 'RUB', 'ru'), String(pt.qty), money(pt.total, 'RUB', 'ru'), pt.note]),
      });
    }
    await downloadPDF($t('nav.printers'), sections);
  }
</script>

{#if canWrite($user) && !editId && !isNew}
  <div class="row no-print"><button onclick={startAdd}>{$t('printer.add')}</button></div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

{#if editId || isNew}
  <div class="card no-print">
    <div class="row">
      <label>{$t('printer.name')}<input bind:value={form.Name} /></label>
      <label>{$t('printer.model')}<input bind:value={form.Model} /></label>
      <label>{$t('printer.price')}<input placeholder="47454" type="number" min="0" bind:value={form.PurchasePrice} /></label>
    </div>
    <div class="row">
      <label>{$t('printer.date')}<input type="date" bind:value={form.CommissioningDate} /></label>
      <label>{$t('printer.life')}<input placeholder="3" type="number" min="1" bind:value={form.LifeYears} /></label>
      <label>{$t('printer.hpd')}<input placeholder="8" type="number" min="1" max="24" bind:value={form.HoursPerDay} /></label>
    </div>
    <div class="row">
      <label>{$t('printer.method')}
        <select bind:value={form.AmortMethod}>
          <option value="calendar">calendar</option>
          <option value="workdays">workdays</option>
          <option value="resource">resource</option>
        </select>
      </label>
      <label>{$t('printer.power')}<input placeholder="0.35" type="number" min="0" step="0.01" bind:value={form.PowerKW} /></label>
      <label>{$t('printer.maint')}<input placeholder="0" type="number" min="0" bind:value={form.MaintPerHour} /></label>
      <label>{$t('printer.resource')}<input placeholder="8000" type="number" min="0" bind:value={form.ResourceHours} /></label>
    </div>
    <div class="row">
      <button onclick={save}>{$t('common.save')}</button>
      <button onclick={() => { editId = 0; isNew = false; }}>{$t('common.cancel')}</button>
    </div>
  </div>
{/if}

<div class="twrap" bind:this={wrapEl}>
{#if dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
<table class="tw" class:dragging={dragKey != null}>
  <thead><tr>
    {#each printerOrd as k}
      <th class:n={printerNum(k)} style={thWidth('printers', k)} data-col={k} class:dragsrc={dragKey === k}>
        {#if canWrite($user)}<span class="grip no-print" role="button" tabindex={0} title={$t('hist.drag_cols')} onpointerdown={(e) => startColDrag(e, wrapEl, k, printerOrd, (dk, x) => { dragKey = dk; dropX = x; }, dropCols)} onkeydown={(e) => colKeyMove(e, printerOrd, k, dropCols)}>⋮⋮</span>{/if}
        <button class="th" onclick={() => toggleSort(k)}>{$t(printerLabel(k))}{arrow(k)}</button>
        <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'printers', k)} ondblclick={() => resetColWidth('printers', k)}></span>
      </th>
    {/each}
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each shown as p}
      <tr>
        {#each printerOrd as k}<td class:n={printerNum(k)}>{printerText(p, k)}</td>{/each}
        <td class="no-print">{#if canWrite($user)}<button onclick={() => startEdit(p)} aria-label={$t('common.edit')}>✎</button> <button onclick={() => remove(p)} aria-label={$t('common.delete_q')}>✕</button>{/if}</td>
      </tr>
    {/each}
  </tbody>
</table>
</div>

<h3>{$t('moon.title')}</h3>
{#each printers as p}
  <div class="card">
    <b>{p.Name}</b>
    {#if canWrite($user)}
      <div class="row">
        <input placeholder="http://192.168.1.50:7125" bind:value={urls[p.ID]} />
        <button onclick={() => saveURL(p)}>{$t('common.save')}</button>
        <button onclick={() => ping(p)}>{$t('moon.check')}</button>
        <button onclick={() => preview(p)}>{$t('moon.import')}</button>
        <input type="number" min="1" step="1" bind:value={moonLimit[p.ID]} placeholder={$t('moon.limit')} title={$t('moon.limit')} style={`width:${$t('moon.limit').length + 5}ch`} />
      </div>
    {/if}
    {#if status[p.ID]}<p class="ok">Moonraker {status[p.ID]}</p>{/if}
    {#if report[p.ID]}<p>{$t('set.csv_preview')}: total {report[p.ID]!.total}{#if report[p.ID]!.lifetimeTotal} / {$t('moon.lifetime_total')}: {report[p.ID]!.lifetimeTotal}{/if}, new {report[p.ID]!.new}, existing {report[p.ID]!.existing}</p>{/if}
    {#if report[p.ID]?.dryRun && canWrite($user)}<button class="warn no-print" onclick={() => apply(p)} disabled={importing[p.ID]}>{$t('set.csv_apply')}</button>{/if}
    {#if importing[p.ID] && progress[p.ID]}<progress value={progress[p.ID].done} max={progress[p.ID].total || 1}></progress> {progress[p.ID].done}/{progress[p.ID].total}{/if}
    {#if applied[p.ID]}<p class="ok">{$t('set.csv_done')}: +{applied[p.ID]!.new} (stubs {applied[p.ID]!.stubs}, {$t('set.csv_spool_stubs')} {applied[p.ID]!.spoolStubs ?? 0})</p>{/if}
    <h4>{$t('part.title')}</h4>
    <table>
      <thead><tr><th><button class="th" onclick={() => togglePartSort('date')}>{$t('part.date')}{partArrow('date')}</button></th><th><button class="th" onclick={() => togglePartSort('name')}>{$t('part.name')}{partArrow('name')}</button></th><th class="n"><button class="th" onclick={() => togglePartSort('price')}>{$t('part.price')}{partArrow('price')}</button></th><th class="n"><button class="th" onclick={() => togglePartSort('qty')}>{$t('part.qty')}{partArrow('qty')}</button></th><th class="n"><button class="th" onclick={() => togglePartSort('total')}>{$t('part.total')}{partArrow('total')}</button></th><th><button class="th" onclick={() => togglePartSort('note')}>{$t('part.note')}{partArrow('note')}</button></th><th class="no-print"></th></tr></thead>
      <tbody>
        {#each sortedParts(p.ID) as pt}
          {#if partEdit && partEdit.id === pt.id}
            <tr>
              <td><input type="date" bind:value={partEdit.boughtAt} /></td>
              <td><input bind:value={partEdit.name} placeholder={$t('part.name')} /></td>
              <td><input type="number" min="0" step="0.01" bind:value={partEdit.price} placeholder="500" /></td>
              <td><input type="number" min="0" step="0.5" bind:value={partEdit.qty} placeholder="1" /></td>
              <td class="n">{money(Math.round(((partEdit.price ?? 0) * (partEdit.qty ?? 1)) * 100) / 100, 'RUB', 'ru')}</td>
              <td><input bind:value={partEdit.note} placeholder={$t('part.note')} /></td>
              <td class="no-print"><button onclick={savePart}>{$t('common.save')}</button> <button onclick={() => (partEdit = null)}>{$t('common.cancel')}</button></td>
            </tr>
          {:else}
            <tr><td>{pt.boughtAt || '—'}</td><td>{pt.name}</td><td class="n">{money(pt.price, 'RUB', 'ru')}</td><td class="n">{pt.qty}</td><td class="n">{money(pt.total, 'RUB', 'ru')}</td><td>{pt.note}</td>
              <td class="no-print">{#if canWrite($user)}<button onclick={() => editPart(pt)} aria-label={$t('common.edit')}>✎</button> <button onclick={() => removePart(pt)} aria-label={$t('common.delete_q')}>✕</button>{/if}</td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
    {#if canWrite($user)}
      {#if partEdit && partEdit.id === 0 && partEdit.printerId === p.ID}
        <div class="row">
          <input type="date" bind:value={partEdit.boughtAt} />
          <input bind:value={partEdit.name} placeholder={$t('part.name')} />
          <input type="number" min="0" step="0.01" bind:value={partEdit.price} placeholder="500" />
          <input type="number" min="0" step="0.5" bind:value={partEdit.qty} placeholder="1" />
          <input bind:value={partEdit.note} placeholder={$t('part.note')} />
          <button onclick={savePart}>{$t('common.save')}</button>
          <button onclick={() => (partEdit = null)}>{$t('common.cancel')}</button>
        </div>
      {:else if !partEdit}
        <div class="row"><button onclick={() => newPart(p.ID)}>+ {$t('part.add')}</button></div>
      {/if}
    {/if}
  </div>
{/each}

<style>
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 10px; margin-bottom: 8px; }
  h4 { margin: 10px 0 6px; font-size: 13px; color: var(--muted); }
  .row { display: flex; gap: 6px; margin: 6px 0; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 120px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; margin-bottom: 12px; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 14px; }
  button.warn { border-color: var(--err); color: var(--err); }
  .ok { color: green; }
  .err { color: var(--err); }
</style>
