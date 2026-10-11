<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MaterialDTO, type Material, type SpoolDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { notify } from '../lib/notice';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';
  import { colKeyMove } from '../lib/colkeyboard';

  let materials: MaterialDTO[] = $state([]);
  let error = $state('');
  // Форма добавления: null — пусто, виден плейсхолдер/title. Цены здесь нет —
  // она живёт только на катушках (добавляются отдельно после создания).
  let newCode = $state('');
  let newType = $state('PETG');
  let newDensity: number | null = $state(null);
  let newWeight: number | null = $state(null);
  let newVendor = $state('');
  let newColor = $state('#808080');
  let newNote = $state('');
  // Редактирование материала (цены здесь нет — она живёт на катушках).
  let editId = $state(0);
  let edit: Material = $state({ ID: 0, Code: '', Type: 'PETG', Vendor: '', ColorHex: '', Density: 1.27, Diameter: 1.75, DefaultSpoolWeightG: 1000, Note: '' });

  // Катушки материала: список, раскрытие, правка.
  // Цена для расчётов — первая неархивная (порядок ListSpools); бейдж «расчётная».
  type SpoolForm = { id: number; materialId: number; boughtAt: string; price: number | null; weight: number | null; currency: string; archived: boolean };
  let spools: Record<number, SpoolDTO[]> = $state({});
  let expanded = $state(0);
  let spoolEdit: SpoolForm | null = $state(null);

  function calcSpool(list: SpoolDTO[]): SpoolDTO | null {
    return list.find((s) => !s.archived) ?? null;
  }

  async function toggleSpools(m: MaterialDTO) {
    if (expanded === m.id) {
      expanded = 0;
      return;
    }
    expanded = m.id;
    await loadSpools(m.id);
  }

  async function loadSpools(materialId: number) {
    try {
      spools[materialId] = (await wails.getSpools(materialId)) ?? [];
    } catch (e) {
      error = String(e);
    }
  }

  function newSpool(m: MaterialDTO) {
    spoolEdit = { id: 0, materialId: m.id, boughtAt: '', price: null, weight: null, currency: 'RUB', archived: false };
  }

  // Подсветка изменённой строки (вспышка гаснет сама через 650мс).
  // Ключи: 'm:<id>' — материал, 's:<id>' — катушка.
  let flashKey: string | null = $state(null);
  let flashTimer: number | undefined;
  function flash(key: string) {
    window.clearTimeout(flashTimer);
    flashKey = key;
    flashTimer = window.setTimeout(() => { flashKey = null; }, 650);
  }

  function editSpool(s: SpoolDTO) {
    spoolEdit = { id: s.id, materialId: s.materialId, boughtAt: s.boughtAt, price: s.price, weight: s.weightInitG, currency: s.currency, archived: s.archived };
  }

  async function saveSpool(defWeight: number) {
    const se = spoolEdit;
    if (!se) return;
    error = '';
    try {
      if (se.id === 0) {
        const w = se.weight ?? defWeight;
        const sid = await wails.addSpool({
          id: 0, materialId: se.materialId, spoolNo: 0, price: se.price ?? 0,
          currency: se.currency || 'RUB', boughtAt: se.boughtAt,
          weightInitG: w, weightLeftG: w, archived: false,
        });
        flash('s:' + sid);
      } else {
        const cur = (spools[se.materialId] ?? []).find((s) => s.id === se.id);
        if (!cur) return;
        await wails.updateSpool({
          ID: cur.id, MaterialID: cur.materialId, Price: Math.round((se.price ?? 0) * 100),
          Currency: se.currency || cur.currency, FxRate: 1, BoughtAt: se.boughtAt,
          WeightInitG: se.weight ?? cur.weightInitG, WeightLeftG: cur.weightLeftG,
          Archived: se.archived,
        });
        flash('s:' + se.id);
      }
      const mid = se.materialId;
      spoolEdit = null;
      await loadSpools(mid);
      await load();
      notify($t('common.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function removeSpool(s: SpoolDTO) {
    error = '';
    if (!(await askConfirm(`${$t('common.delete_q')} «${s.boughtAt || s.id}»?`))) return;
    try {
      await wails.deleteSpool(s.id);
      await loadSpools(s.materialId);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  let knownTypes = $derived([...new Set(materials.map((m) => m.type))].sort());
  // Цвета: пресеты + фавориты (settings).
  const presets = [
    '#000000', '#ffffff', '#808080', '#ff0000', '#ff8000', '#ffff00',
    '#00c000', '#00c0ff', '#0000ff', '#8000ff', '#ff40c0', '#804000',
  ];
  let favs: string[] = $state([]);
  // Сортировка по клику на заголовок.
  let sortKey = $state('code');
  let sortDir: 1 | -1 = $state(1);

  function toggleSort(key: string) {
    if (sortKey === key) sortDir = sortDir === 1 ? -1 : 1;
    else { sortKey = key; sortDir = 1; }
  }

  function arrow(key: string): string {
    return sortKey === key ? (sortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  function sortVal(m: MaterialDTO): string | number {
    switch (sortKey) {
      case 'code': return m.code.toLowerCase();
      case 'type': return (m.type || '').toLowerCase();
      case 'price': return m.avgPricePerKg;
      case 'density': return m.density;
      case 'weight': return m.defaultWeightG || 1000;
      case 'vendor': return (m.vendor || '').toLowerCase();
      case 'note': return (m.note || '').toLowerCase();
      default: return m.code.toLowerCase();
    }
  }

  let shown = $derived([...materials].sort((a, b) => {
    const va = sortVal(a);
    const vb = sortVal(b);
    const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
    return cmp * sortDir;
  }));

  const matCols = [
    { key: 'code', label: 'mat.code' }, { key: 'color', label: 'mat.color' },
    { key: 'type', label: 'mat.type' }, { key: 'price', label: 'mat.avg_price', num: true },
    { key: 'density', label: 'mat.density', num: true }, { key: 'weight', label: 'mat.weight', num: true },
    { key: 'vendor', label: 'mat.vendor' }, { key: 'note', label: 'mat.note' },
  ];
  const matKeys = matCols.map((c) => c.key);
  const matNum = (k: string) => matCols.some((c) => c.key === k && c.num);
  const matLabel = (k: string) => matCols.find((c) => c.key === k)?.label ?? k;

  let matOrd = $derived(orderFor($mySettings.colOrder, 'materials', matKeys));

  const spoolCols = [
    { key: 'num', label: 'spool.num' }, { key: 'date', label: 'spool.date' },
    { key: 'price', label: 'spool.price', num: true },
    { key: 'weight', label: 'spool.weight', num: true },
    { key: 'left', label: 'spool.left', num: true },
    { key: 'flag', label: '' },
  ];
  const spoolKeys = spoolCols.map((c) => c.key);
  const spoolNum = (k: string) => spoolCols.some((c) => c.key === k && c.num);
  const spoolLabel = (k: string) => spoolCols.find((c) => c.key === k)?.label ?? k;

  let spoolOrd = $derived(orderFor($mySettings.colOrder, 'spools', spoolKeys));

  // Текст ячейки катушки (flag — маркеры ★/архив, шапка пустая).
  function spoolText(s: SpoolDTO, list: SpoolDTO[], k: string): string {
    switch (k) {
      case 'num': return String(s.spoolNo || '—');
      case 'date': return s.boughtAt || '—';
      case 'price': return money(s.price, s.currency, 'ru');
      case 'weight': return String(s.weightInitG);
      // Остаток — вычисляемый (партия минус расход по истории), не поле БД.
      case 'left': return String(Math.round(s.leftG ?? s.weightLeftG));
      case 'flag': {
        const star = calcSpool(list)?.id === s.id ? `★ ${$t('spool.calc')}` : '';
        const arch = s.archived ? ` · ${$t('spool.archived')}` : '';
        return star + arch;
      }
      default: return '';
    }
  }

  // Текстовое значение ячейки (цвет — отдельно квадратиком).
  function matText(m: MaterialDTO, k: string): string {
    switch (k) {
      case 'code': return m.code;
      case 'type': return m.type;
      case 'price': return money(m.avgPricePerKg, m.currency, 'ru');
      case 'density': return String(m.density);
      case 'weight': return String(m.defaultWeightG || 1000);
      case 'vendor': return m.vendor;
      case 'note': return m.note;
      default: return '';
    }
  }

  // Drag-перестановка за грип.
  let dragKey: string | null = $state(null);
  let dropX: number | null = $state(null);
  let wrapEl: HTMLDivElement | null = $state(null);

  async function dropCols(next: string[]) {
    error = '';
    try {
      await saveTableOrder('materials', next);
    } catch (e) {
      error = String(e);
    }
  }

  // Катушки: свой wrap/drag (ключ таблицы 'spools'). Отдельный от материалов,
  // т.к. coldrag ищет th[data-col] внутри wrap — общий wrap смешал бы списки.
  let spoolDragKey: string | null = $state(null);
  let spoolDropX: number | null = $state(null);
  let spoolWrapEl: HTMLDivElement | null = $state(null);

  async function dropSpoolCols(next: string[]) {
    error = '';
    try {
      await saveTableOrder('spools', next);
    } catch (e) {
      error = String(e);
    }
  }

  function validHex(s: string): boolean {
    return /^#[0-9a-fA-F]{6}$/.test(s.trim());
  }

  function pickColor(hex: string) {
    if (validHex(hex)) edit.ColorHex = hex.toLowerCase();
  }

  async function toggleFav(hex: string) {
    hex = hex.toLowerCase();
    const next = favs.includes(hex) ? favs.filter((c) => c !== hex) : [...favs, hex];
    try {
      await wails.setMaterialColorFavs(next);
      favs = next;
    } catch (e) {
      error = String(e);
    }
  }

  async function load() {
    try {
      materials = (await wails.materials()) ?? [];
      favs = (await wails.getMaterialColorFavs()) ?? [];
    } catch (e) {
      error = String(e);
    }
  }
  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
  });

  async function add() {
    error = '';
    try {
      if (newColor && !validHex(newColor)) {
        error = $t('mat.bad_color');
        return;
      }
      const newId = await wails.addMaterial(
        newCode.trim(), newType.trim() || 'PETG', newDensity ?? 1.27, newWeight ?? 1000,
        newVendor.trim(), (newColor || '').toLowerCase(), newNote.trim(),
      );
      newCode = '';
      newDensity = null;
      newWeight = null;
      newVendor = '';
      newColor = '#808080';
      newNote = '';
      await load();
      notify($t('common.saved'));
      if (newId) flash('m:' + newId);
    } catch (e) {
      error = String(e);
    }
  }

  function startEdit(m: MaterialDTO) {
    editId = m.id;
    edit = { ID: m.id, Code: m.code, Type: m.type, Vendor: m.vendor, ColorHex: m.colorHex, Density: m.density, Diameter: m.diameter, DefaultSpoolWeightG: m.defaultWeightG || 1000, Note: m.note };
  }

  function swatch(hex: string): string {
    return validHex(hex || '') ? hex : 'transparent';
  }

  async function saveEdit() {
    error = '';
    try {
      if (edit.ColorHex && !validHex(edit.ColorHex)) {
        error = $t('mat.bad_color');
        return;
      }
      await wails.updateMaterial(edit);
      const savedId = editId;
      editId = 0;
      await load();
      notify($t('common.saved'));
      if (savedId) flash('m:' + savedId);
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(m: MaterialDTO) {
    error = '';
    if (!(await askConfirm(`${$t("common.delete_q")} «${m.code}»?`))) return;
    try {
      await wails.deleteMaterial(m.id);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function toPDF() {
    const sections = [
      {
        Title: '',
        Headers: matOrd.map((k) => $t(matLabel(k))),
        Rows: materials.map((m) => matOrd.map((k) => (k === 'color' ? m.colorHex : matText(m, k)))),
      },
    ];
    for (const m of materials) {
      const list = spools[m.id];
      if (expanded !== m.id || !list || !list.length) continue;
      sections.push({
        Title: `${m.code} — ${$t('spool.title')}`,
        Headers: spoolOrd.map((k) => (spoolLabel(k) ? $t(spoolLabel(k)) : '')),
        Rows: list.map((s) => spoolOrd.map((k) => spoolText(s, list, k))),
      });
    }
    await downloadPDF($t('nav.materials'), sections);
  }
</script>

{#if canWrite($user)}
  <div class="row no-print">
    <input placeholder="PETG-NIT-W4" title={$t('mat.code')} bind:value={newCode} />
    <input type="color" value={validHex(newColor) ? newColor : '#808080'} oninput={(e) => (newColor = e.currentTarget.value)} title={$t('mat.color')} />
    <input class="hex" bind:value={newColor} placeholder="#rrggbb" title={$t('mat.color')} />
    <input list="mat-types" bind:value={newType} placeholder={$t('mat.type')} />
    <datalist id="mat-types">
      {#each knownTypes as tp}<option value={tp}></option>{/each}
    </datalist>
    <input type="number" step="0.01" bind:value={newDensity} title={$t('mat.density')} placeholder="1.27" />
    <input type="number" min="0" bind:value={newWeight} title={$t('mat.weight')} placeholder="1000" />
    <input bind:value={newVendor} placeholder={$t('mat.vendor')} />
    <input bind:value={newNote} placeholder={$t('mat.note')} />
    <button onclick={add}>{$t('common.save')}</button>
  </div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

<div class="twrap" bind:this={wrapEl}>
{#if dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
<table class="tw" class:dragging={dragKey != null}>
  <thead><tr>
    {#each matOrd as k}
      <th class:n={matNum(k)} style={thWidth('materials', k)} data-col={k} class:dragsrc={dragKey === k}>
        {#if canWrite($user)}<span class="grip no-print" role="button" tabindex={0} title={$t('hist.drag_cols')} onpointerdown={(e) => { if (canWrite($user)) startColDrag(e, wrapEl, k, matOrd, (dk, x) => { dragKey = dk; dropX = x; }, dropCols); }} onkeydown={(e) => colKeyMove(e, matOrd, k, dropCols)}>⋮⋮</span>{/if}
        {#if k === 'color'}
          {$t(matLabel(k))}
        {:else}
          <button class="th" onclick={() => toggleSort(k)}>{$t(matLabel(k))}{arrow(k)}</button>
        {/if}
        <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'materials', k)} ondblclick={() => resetColWidth('materials', k)}></span>
      </th>
    {/each}
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each shown as m}
      {#if editId === m.id}
        <tr>
          {#each matOrd as k}
            {#if k === 'code'}
              <td><input bind:value={edit.Code} placeholder="PETG-NIT-BLACK" title={$t('mat.code_rename_note')} /></td>
            {:else if k === 'color'}
              <td>
                <div class="colorrow">
                  <input type="color" value={validHex(edit.ColorHex) ? edit.ColorHex : '#000000'} oninput={(e) => (edit.ColorHex = e.currentTarget.value)} title={$t('mat.color')} />
                  <input class="hex" bind:value={edit.ColorHex} placeholder="#rrggbb" />
                </div>
                <div class="colorrow">
                  {#each favs as f}<button class="sw" style="background:{f}" title={f} onclick={() => pickColor(f)}></button>{/each}
                  {#each presets as p}<button class="sw preset" style="background:{p}" title={p} onclick={() => pickColor(p)}></button>{/each}
                </div>
                {#if canWrite($user)}
                  <div class="colorrow">
                    <button onclick={() => edit.ColorHex && toggleFav(edit.ColorHex)}>{favs.includes(edit.ColorHex.toLowerCase()) ? '★' : '☆'} {$t('mat.fav')}</button>
                  </div>
                {/if}
              </td>
            {:else if k === 'type'}
              <td><input list="mat-types-edit" bind:value={edit.Type} placeholder="PETG" /></td>
            {:else if k === 'price'}
              <td class="n" title={$t('mat.avg_note')}>{matText(m, k)}</td>
            {:else if k === 'density'}
              <td><input placeholder="1.27" type="number" step="0.01" bind:value={edit.Density} /></td>
            {:else if k === 'weight'}
              <td><input type="number" min="0" step="1" bind:value={edit.DefaultSpoolWeightG} title={$t('mat.weight')} /></td>
            {:else if k === 'vendor'}
              <td><input bind:value={edit.Vendor} placeholder={$t('mat.vendor')} /></td>
            {:else if k === 'note'}
              <td><input bind:value={edit.Note} placeholder={$t('mat.note')} /></td>
            {/if}
          {/each}
          <td class="no-print"><button onclick={saveEdit}>{$t('common.save')}</button> <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button></td>
        </tr>
      {:else}
        <tr class:row-flash={flashKey === 'm:' + m.id}>
          {#each matOrd as k}
            {#if k === 'color'}
              <td><span class="sw" style="background:{swatch(m.colorHex)}" title={m.colorHex}></span></td>
            {:else}
              <td class:n={matNum(k)}>{matText(m, k)}</td>
            {/if}
          {/each}
          <td class="no-print">
            <button onclick={() => toggleSpools(m)} title={$t('spool.title')}>{expanded === m.id ? '▾' : '▸'} {(spools[m.id] ?? []).length || ''}</button>
            {#if canWrite($user)}<button onclick={() => startEdit(m)} aria-label={$t('common.edit')}>✎</button> <button onclick={() => remove(m)} aria-label={$t('common.delete_q')}>✕</button>{/if}
          </td>
        </tr>
      {/if}
      {#if expanded === m.id}
        <tr><td colspan={matOrd.length + 1}>
          <b>{$t('spool.title')} — {m.code}</b>
          <div class="twrap" bind:this={spoolWrapEl}>
          {#if spoolDropX != null}<div class="drop-line no-print" style="left:{spoolDropX}px;"></div>{/if}
          <table class="tw">
            <thead><tr>
              {#each spoolOrd as k}
                <th class:n={spoolNum(k)} style={thWidth('spools', k)} data-col={k} class:dragsrc={spoolDragKey === k}>
                  {#if canWrite($user)}<span class="grip no-print" role="button" tabindex={0} title={$t('hist.drag_cols')} onpointerdown={(e) => { if (canWrite($user)) startColDrag(e, spoolWrapEl, k, spoolOrd, (dk, x) => { spoolDragKey = dk; spoolDropX = x; }, dropSpoolCols); }} onkeydown={(e) => colKeyMove(e, spoolOrd, k, dropSpoolCols)}>⋮⋮</span>{/if}
                  {#if spoolLabel(k)}<span>{$t(spoolLabel(k))}</span>{/if}
                  <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'spools', k)} ondblclick={() => resetColWidth('spools', k)}></span>
                </th>
              {/each}
              <th class="no-print"></th>
            </tr></thead>
            <tbody>
              {#each spools[m.id] ?? [] as s}
                {#if spoolEdit && spoolEdit.id === s.id}
                  <tr>
                    {#each spoolOrd as k}
                      {#if k === 'num'}
                        <td>{s.spoolNo || '—'}</td>
                      {:else if k === 'date'}
                        <td><input type="date" bind:value={spoolEdit.boughtAt} /></td>
                      {:else if k === 'price'}
                        <td><input type="number" min="0" step="0.01" bind:value={spoolEdit.price} placeholder="869" /></td>
                      {:else if k === 'weight'}
                        <td><input type="number" min="0" step="1" bind:value={spoolEdit.weight} placeholder="1000" /></td>
                      {:else if k === 'left'}
                        <td class="n">{Math.round(s.leftG ?? s.weightLeftG)}</td>
                      {:else if k === 'flag'}
                        <td><input type="checkbox" bind:checked={spoolEdit.archived} title={$t('spool.archived')} />{#if calcSpool(spools[m.id] ?? [])?.id === s.id}★ {$t('spool.calc')}{/if}</td>
                      {/if}
                    {/each}
                    <td class="no-print"><button onclick={() => saveSpool(m.defaultWeightG || 1000)}>{$t('common.save')}</button> <button onclick={() => (spoolEdit = null)}>{$t('common.cancel')}</button></td>
                  </tr>
                {:else}
                  <tr class:row-flash={flashKey === 's:' + s.id}>
                    {#each spoolOrd as k}
                      <td class:n={spoolNum(k)}>{spoolText(s, spools[m.id] ?? [], k)}</td>
                    {/each}
                    <td class="no-print">{#if canWrite($user)}<button onclick={() => editSpool(s)} aria-label={$t('common.edit')}>✎</button> <button onclick={() => removeSpool(s)} aria-label={$t('common.delete_q')}>✕</button>{/if}</td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
          </div>
          {#if canWrite($user)}
            {#if spoolEdit && spoolEdit.id === 0 && spoolEdit.materialId === m.id}
              <div class="row">
                <input type="date" bind:value={spoolEdit.boughtAt} title={$t('spool.date')} />
                <input type="number" min="0" step="0.01" bind:value={spoolEdit.price} placeholder="869" title={$t('spool.price')} />
                <input type="number" min="0" step="1" bind:value={spoolEdit.weight} placeholder="1000" title={$t('spool.weight')} />
                <button onclick={() => saveSpool(m.defaultWeightG || 1000)}>{$t('common.save')}</button>
                <button onclick={() => (spoolEdit = null)}>{$t('common.cancel')}</button>
              </div>
            {:else if !spoolEdit}
              <div class="row"><button onclick={() => newSpool(m)}>+ {$t('spool.add')}</button></div>
            {/if}
          {/if}
        </td></tr>
      {/if}
    {/each}
  </tbody>
</table>
</div>
<datalist id="mat-types-edit">
  {#each knownTypes as tp}<option value={tp}></option>{/each}
</datalist>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
  input, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  .err { color: var(--err); }
  .sw { display: inline-block; width: 18px; height: 18px; border: 1px solid var(--sw-border); border-radius: 3px; padding: 0; vertical-align: middle; print-color-adjust: exact; }
  button.sw { cursor: pointer; margin: 1px; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 14px; }
  .colorrow { display: flex; gap: 4px; align-items: center; margin-bottom: 4px; flex-wrap: wrap; }
  input[type='color'] { width: 40px; height: 30px; padding: 2px; }
  input.hex { width: 90px; font-family: monospace; }
</style>
