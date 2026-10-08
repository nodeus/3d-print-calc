<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MaterialDTO, type Material } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';

  let materials: MaterialDTO[] = $state([]);
  let error = $state('');
  // Форма добавления
  let newCode = $state('');
  let newType = $state('PETG');
  let newDensity = $state(1.27);
  let newPrice = $state(0);
  let newWeight = $state(1000);
  // Редактирование
  let editId = $state(0);
  let edit: Material = $state({ ID: 0, Code: '', Type: 'PETG', Vendor: '', ColorHex: '', Density: 1.27, Diameter: 1.75, DefaultSpoolWeightG: 1000, Note: '' });
  let editPrice = $state(0);

  let knownTypes = $derived([...new Set(materials.map((m) => m.type))].sort());
  // Цвета: пресеты + фавориты (settings).
  const presets = [
    '#000000', '#ffffff', '#808080', '#ff0000', '#ff8000', '#ffff00',
    '#00c000', '#00c0ff', '#0000ff', '#8000ff', '#ff40c0', '#804000',
  ];
  let favs: string[] = $state([]);

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
      await wails.addMaterial(newCode.trim(), newType.trim() || 'PETG', newDensity, newPrice, newWeight);
      newCode = '';
      newPrice = 0;
      newWeight = 1000;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  function startEdit(m: MaterialDTO) {
    editId = m.id;
    edit = { ID: m.id, Code: m.code, Type: m.type, Vendor: m.vendor, ColorHex: m.colorHex, Density: m.density, Diameter: m.diameter, DefaultSpoolWeightG: m.defaultWeightG || 1000, Note: m.note };
    editPrice = m.pricePerKg;
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
      // Цена — отдельным вызовом: создаст катушку, если её нет, остальное не трогает.
      await wails.setMaterialPrice(editId, Math.round(editPrice * 100), 'RUB');
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
    await downloadPDF($t('nav.materials'), [
      {
        Title: '',
        Headers: [$t('mat.code'), $t('mat.color'), $t('mat.type'), $t('mat.price'), $t('mat.density'), $t('mat.weight'), $t('mat.vendor')],
        Rows: materials.map((m) => [m.code, m.colorHex, m.type, money(m.pricePerKg, m.currency, 'ru'), String(m.density), String(m.defaultWeightG || 1000), m.vendor]),
      },
    ]);
  }
</script>

{#if canWrite($user)}
  <div class="row no-print">
    <input placeholder="PETG-NIT-W4" bind:value={newCode} />
    <input list="mat-types" bind:value={newType} placeholder={$t('mat.type')} />
    <datalist id="mat-types">
      {#each knownTypes as tp}<option value={tp}></option>{/each}
    </datalist>
    <input type="number" step="0.01" bind:value={newDensity} title={$t('mat.density')} />
    <input type="number" min="0" bind:value={newPrice} title={$t('mat.price')} />
    <input type="number" min="0" bind:value={newWeight} title={$t('mat.weight')} />
    <button onclick={add}>{$t('common.save')}</button>
  </div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

<table>
  <thead><tr><th>{$t('mat.code')}</th><th>{$t('mat.color')}</th><th>{$t('mat.type')}</th><th>{$t('mat.price')}</th><th>{$t('mat.density')}</th><th class="n">{$t('mat.weight')}</th><th>{$t('mat.vendor')}</th><th class="no-print"></th></tr></thead>
  <tbody>
    {#each materials as m}
      {#if editId === m.id}
        <tr>
          <td>{m.code}</td>
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
          <td><input list="mat-types-edit" bind:value={edit.Type} placeholder="PETG" /></td>
          <td><input placeholder="869" type="number" min="0" bind:value={editPrice} /></td>
          <td><input placeholder="1.27" type="number" step="0.01" bind:value={edit.Density} /></td>
          <td><input type="number" min="0" step="1" bind:value={edit.DefaultSpoolWeightG} title={$t('mat.weight')} /></td>
          <td><input bind:value={edit.Vendor} /></td>
          <td class="no-print"><button onclick={saveEdit}>{$t('common.save')}</button> <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button></td>
        </tr>
      {:else}
        <tr><td>{m.code}</td><td><span class="sw" style="background:{swatch(m.colorHex)}" title={m.colorHex}></span></td><td>{m.type}</td><td class="n">{money(m.pricePerKg, m.currency, 'ru')}</td><td class="n">{m.density}</td><td class="n">{m.defaultWeightG || 1000}</td><td>{m.vendor}</td>
          <td class="no-print">
            {#if canWrite($user)}<button onclick={() => startEdit(m)}>✎</button> <button onclick={() => remove(m)}>✕</button>{/if}
          </td>
        </tr>
      {/if}
    {/each}
  </tbody>
</table>
<datalist id="mat-types-edit">
  {#each knownTypes as tp}<option value={tp}></option>{/each}
</datalist>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
  input, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  .err { color: var(--err); }
  .sw { display: inline-block; width: 18px; height: 18px; border: 1px solid var(--sw-border); border-radius: 3px; padding: 0; vertical-align: middle; }
  button.sw { cursor: pointer; margin: 1px; }
  .colorrow { display: flex; gap: 4px; align-items: center; margin-bottom: 4px; flex-wrap: wrap; }
  input[type='color'] { width: 40px; height: 30px; padding: 2px; }
  input.hex { width: 90px; font-family: monospace; }
</style>
