<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MaterialDTO, type Material } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { money } from '../lib/format';

  let materials: MaterialDTO[] = $state([]);
  let error = $state('');
  // Форма добавления
  let newCode = $state('');
  let newType = $state('PETG');
  let newDensity = $state(1.27);
  let newPrice = $state(0);
  // Редактирование
  let editId = $state(0);
  let edit: Material = $state({ ID: 0, Code: '', Type: 'PETG', Vendor: '', ColorHex: '', Density: 1.27, Diameter: 1.75, Note: '' });
  let editPrice = $state(0);

  async function load() {
    try {
      materials = await wails.materials();
    } catch (e) {
      error = String(e);
    }
  }
  onMount(load);

  async function add() {
    error = '';
    try {
      await wails.addMaterial(newCode.trim(), newType, newDensity, newPrice);
      newCode = '';
      newPrice = 0;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  function startEdit(m: MaterialDTO) {
    editId = m.id;
    edit = { ID: m.id, Code: m.code, Type: m.type, Vendor: m.vendor, ColorHex: m.colorHex, Density: m.density, Diameter: m.diameter, Note: m.note };
    editPrice = m.pricePerKg;
  }

  async function saveEdit() {
    error = '';
    try {
      await wails.updateMaterial(edit);
      if (editPrice >= 0) {
        const m = materials.find((x) => x.id === editId);
        if (m && m.spoolId) {
          await wails.updateSpool({
            ID: m.spoolId, MaterialID: m.id, Price: Math.round(editPrice * 100),
            Currency: m.currency, FxRate: 1, BoughtAt: '', WeightInitG: 1000,
            WeightLeftG: m.spoolLeftG || 1000, Archived: false,
          });
        }
      }
      editId = 0;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(m: MaterialDTO) {
    error = '';
    if (!confirm(`${m.code}?`)) return;
    try {
      await wails.deleteMaterial(m.id);
      await load();
    } catch (e) {
      error = String(e);
    }
  }
</script>

{#if canWrite($user)}
  <div class="row no-print">
    <input placeholder="PETG-NIT-W4" bind:value={newCode} />
    <select bind:value={newType}>
      {#each ['PETG', 'PLA', 'ABS', 'ASA', 'TPU', 'NYLON'] as tp}<option value={tp}>{tp}</option>{/each}
    </select>
    <input type="number" step="0.01" bind:value={newDensity} title={$t('mat.density')} />
    <input type="number" min="0" bind:value={newPrice} title={$t('mat.price')} />
    <button onclick={add}>{$t('common.save')}</button>
  </div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

<table>
  <thead><tr><th>{$t('mat.code')}</th><th>{$t('mat.type')}</th><th>{$t('mat.price')}</th><th>{$t('mat.density')}</th><th>{$t('mat.vendor')}</th><th class="no-print"></th></tr></thead>
  <tbody>
    {#each materials as m}
      {#if editId === m.id}
        <tr>
          <td>{m.code}</td>
          <td><select bind:value={edit.Type}>{#each ['PETG', 'PLA', 'ABS', 'ASA', 'TPU', 'NYLON'] as tp}<option value={tp}>{tp}</option>{/each}</select></td>
          <td><input type="number" min="0" bind:value={editPrice} /></td>
          <td><input type="number" step="0.01" bind:value={edit.Density} /></td>
          <td><input bind:value={edit.Vendor} /></td>
          <td class="no-print"><button onclick={saveEdit}>{$t('common.save')}</button> <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button></td>
        </tr>
      {:else}
        <tr><td>{m.code}</td><td>{m.type}</td><td class="n">{money(m.pricePerKg, m.currency, 'ru')}</td><td class="n">{m.density}</td><td>{m.vendor}</td>
          <td class="no-print">
            {#if canWrite($user)}<button onclick={() => startEdit(m)}>✎</button> <button onclick={() => remove(m)}>✕</button>{/if}
          </td>
        </tr>
      {/if}
    {/each}
  </tbody>
</table>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; }
  .err { color: #b00; }
</style>
