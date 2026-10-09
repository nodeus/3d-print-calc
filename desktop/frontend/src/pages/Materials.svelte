<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MaterialDTO, type Material, type SpoolDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';

  let materials: MaterialDTO[] = $state([]);
  let error = $state('');
  // Форма добавления: null — пусто, виден плейсхолдер/title.
  let newCode = $state('');
  let newType = $state('PETG');
  let newDensity: number | null = $state(null);
  let newPrice: number | null = $state(null);
  let newWeight: number | null = $state(null);
  // Редактирование материала (цены здесь нет — она живёт на катушках).
  let editId = $state(0);
  let edit: Material = $state({ ID: 0, Code: '', Type: 'PETG', Vendor: '', ColorHex: '', Density: 1.27, Diameter: 1.75, DefaultSpoolWeightG: 1000, Note: '', BoughtAt: '', Archived: false });

  // Катушки материала: список, раскрытие, правка.
  // Цена для расчётов — первая неархивная (порядок ListSpools); бейдж «расчётная».
  type SpoolForm = { id: number; materialId: number; boughtAt: string; price: number | null; weight: number | null; currency: string };
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
    spoolEdit = { id: 0, materialId: m.id, boughtAt: '', price: null, weight: null, currency: 'RUB' };
  }

  function editSpool(s: SpoolDTO) {
    spoolEdit = { id: s.id, materialId: s.materialId, boughtAt: s.boughtAt, price: s.price, weight: s.weightInitG, currency: s.currency };
  }

  async function saveSpool(defWeight: number) {
    const se = spoolEdit;
    if (!se) return;
    error = '';
    try {
      if (se.id === 0) {
        const w = se.weight ?? defWeight;
        await wails.addSpool({
          id: 0, materialId: se.materialId, spoolNo: 0, price: se.price ?? 0,
          currency: se.currency || 'RUB', boughtAt: se.boughtAt,
          weightInitG: w, weightLeftG: w, archived: false,
        });
      } else {
        const cur = (spools[se.materialId] ?? []).find((s) => s.id === se.id);
        if (!cur) return;
        await wails.updateSpool({
          ID: cur.id, MaterialID: cur.materialId, Price: Math.round((se.price ?? 0) * 100),
          Currency: se.currency || cur.currency, FxRate: 1, BoughtAt: se.boughtAt,
          WeightInitG: se.weight ?? cur.weightInitG, WeightLeftG: cur.weightLeftG,
          Archived: cur.archived,
        });
      }
      const mid = se.materialId;
      spoolEdit = null;
      await loadSpools(mid);
      await load();
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
      case 'price': return m.pricePerKg;
      case 'density': return m.density;
      case 'weight': return m.defaultWeightG || 1000;
      case 'bought': return m.boughtAt || '';
      case 'archived': return m.archived ? 1 : 0;
      case 'vendor': return (m.vendor || '').toLowerCase();
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
    { key: 'type', label: 'mat.type' }, { key: 'price', label: 'mat.price', num: true },
    { key: 'density', label: 'mat.density', num: true }, { key: 'weight', label: 'mat.weight', num: true },
    { key: 'bought', label: 'mat.bought' }, { key: 'archived', label: 'mat.archived' },
    { key: 'vendor', label: 'mat.vendor' },
  ];
  const matKeys = matCols.map((c) => c.key);
  const matNum = (k: string) => matCols.some((c) => c.key === k && c.num);
  const matLabel = (k: string) => matCols.find((c) => c.key === k)?.label ?? k;

  let matOrd = $derived(orderFor($mySettings.colOrder, 'materials', matKeys));

  // Текстовое значение ячейки (цвет — отдельно квадратиком).
  function matText(m: MaterialDTO, k: string): string {
    switch (k) {
      case 'code': return m.code;
      case 'type': return m.type;
      case 'price': return money(m.pricePerKg, m.currency, 'ru');
      case 'density': return String(m.density);
      case 'weight': return String(m.defaultWeightG || 1000);
      case 'bought': return m.boughtAt || '—';
      case 'archived': return m.archived ? '✓' : '';
      case 'vendor': return m.vendor;
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
      await wails.addMaterial(newCode.trim(), newType.trim() || 'PETG', newDensity ?? 1.27, newPrice ?? 0, newWeight ?? 1000);
      newCode = '';
      newPrice = null;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  function startEdit(m: MaterialDTO) {
    editId = m.id;
    edit = { ID: m.id, Code: m.code, Type: m.type, Vendor: m.vendor, ColorHex: m.colorHex, Density: m.density, Diameter: m.diameter, DefaultSpoolWeightG: m.defaultWeightG || 1000, Note: m.note, BoughtAt: m.boughtAt || '', Archived: m.archived };
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
      editId = 0;
      await load();
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
        Headers: [$t('spool.num'), $t('spool.date'), $t('spool.price'), $t('spool.weight'), $t('spool.left'), ''],
        Rows: list.map((s) => [
          String(s.spoolNo || '—'), s.boughtAt || '—', money(s.price, s.currency, 'ru'),
          String(s.weightInitG), String(s.weightLeftG),
          calcSpool(list)?.id === s.id ? '★' : '',
        ]),
      });
    }
    await downloadPDF($t('nav.materials'), sections);
  }
</script>

{#if canWrite($user)}
  <div class="row no-print">
    <input placeholder="PETG-NIT-W4" bind:value={newCode} />
    <input list="mat-types" bind:value={newType} placeholder={$t('mat.type')} />
    <datalist id="mat-types">
      {#each knownTypes as tp}<option value={tp}></option>{/each}
    </datalist>
    <input type="number" step="0.01" bind:value={newDensity} title={$t('mat.density')} placeholder="1.27" />
    <input type="number" min="0" bind:value={newPrice} title={$t('mat.price')} placeholder="869" />
    <input type="number" min="0" bind:value={newWeight} title={$t('mat.weight')} placeholder="1000" />
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
        {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => { if (canWrite($user)) startColDrag(e, wrapEl, k, matOrd, (dk, x) => { dragKey = dk; dropX = x; }, dropCols); }}>⋮⋮</span>{/if}
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
              <td class="n" title={$t('mat.price_spool_note')}>{matText(m, k)}</td>
            {:else if k === 'density'}
              <td><input placeholder="1.27" type="number" step="0.01" bind:value={edit.Density} /></td>
            {:else if k === 'weight'}
              <td><input type="number" min="0" step="1" bind:value={edit.DefaultSpoolWeightG} title={$t('mat.weight')} /></td>
            {:else if k === 'bought'}
              <td><input type="date" bind:value={edit.BoughtAt} title={$t('mat.bought')} /></td>
            {:else if k === 'archived'}
              <td><input type="checkbox" bind:checked={edit.Archived} title={$t('mat.archived')} /></td>
            {:else if k === 'vendor'}
              <td><input bind:value={edit.Vendor} /></td>
            {/if}
          {/each}
          <td class="no-print"><button onclick={saveEdit}>{$t('common.save')}</button> <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button></td>
        </tr>
      {:else}
        <tr>
          {#each matOrd as k}
            {#if k === 'color'}
              <td><span class="sw" style="background:{swatch(m.colorHex)}" title={m.colorHex}></span></td>
            {:else}
              <td class:n={matNum(k)}>{matText(m, k)}</td>
            {/if}
          {/each}
          <td class="no-print">
            <button onclick={() => toggleSpools(m)} title={$t('spool.title')}>{expanded === m.id ? '▾' : '▸'} {(spools[m.id] ?? []).length || ''}</button>
            {#if canWrite($user)}<button onclick={() => startEdit(m)}>✎</button> <button onclick={() => remove(m)}>✕</button>{/if}
          </td>
        </tr>
      {/if}
      {#if expanded === m.id}
        <tr><td colspan={matOrd.length + 1}>
          <b>{$t('spool.title')} — {m.code}</b>
          <table>
            <thead><tr><th>{$t('spool.num')}</th><th>{$t('spool.date')}</th><th class="n">{$t('spool.price')}</th><th class="n">{$t('spool.weight')}</th><th class="n">{$t('spool.left')}</th><th></th><th class="no-print"></th></tr></thead>
            <tbody>
              {#each spools[m.id] ?? [] as s}
                {#if spoolEdit && spoolEdit.id === s.id}
                  <tr>
                    <td>{s.spoolNo || '—'}</td>
                    <td><input type="date" bind:value={spoolEdit.boughtAt} /></td>
                    <td><input type="number" min="0" step="0.01" bind:value={spoolEdit.price} placeholder="869" /></td>
                    <td><input type="number" min="0" step="1" bind:value={spoolEdit.weight} placeholder="1000" /></td>
                    <td class="n">{s.weightLeftG}</td>
                    <td>{#if calcSpool(spools[m.id] ?? [])?.id === s.id}★ {$t('spool.calc')}{/if}</td>
                    <td class="no-print"><button onclick={() => saveSpool(m.defaultWeightG || 1000)}>{$t('common.save')}</button> <button onclick={() => (spoolEdit = null)}>{$t('common.cancel')}</button></td>
                  </tr>
                {:else}
                  <tr>
                    <td>{s.spoolNo || '—'}</td>
                    <td>{s.boughtAt || '—'}</td>
                    <td class="n">{money(s.price, s.currency, 'ru')}</td>
                    <td class="n">{s.weightInitG}</td>
                    <td class="n">{s.weightLeftG}</td>
                    <td>{#if calcSpool(spools[m.id] ?? [])?.id === s.id}★ {$t('spool.calc')}{/if}</td>
                    <td class="no-print">{#if canWrite($user)}<button onclick={() => editSpool(s)}>✎</button> <button onclick={() => removeSpool(s)}>✕</button>{/if}</td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
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
