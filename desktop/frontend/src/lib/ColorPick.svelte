<script lang="ts">
  import { t } from './i18n';
  import { baseColors } from './colors';

  let {
    value = $bindable(''),
    matType = 'PETG',
    favs = [],
    onToggleFav,
  }: {
    value: string;
    matType?: string;
    favs?: string[];
    onToggleFav?: (hex: string) => void;
  } = $props();

  function validHex(s: string): boolean {
    return /^#[0-9a-fA-F]{6}$/.test((s || '').trim());
  }

  function pick(hex: string) {
    if (validHex(hex)) value = hex.toLowerCase();
  }
</script>

<div class="colorrow">
  <input type="color" value={validHex(value) ? value : '#808080'} oninput={(e) => (value = e.currentTarget.value)} title={$t('mat.color')} />
  <input class="hex" bind:value placeholder="#rrggbb" title={$t('mat.color')} />
</div>
<div class="colorrow">
  {#each baseColors(matType) as b}<button class="sw" style="background:{b}" title={b} onclick={() => pick(b)}></button>{/each}
  {#each favs as f}<button class="sw fav" style="background:{f}" title={f} onclick={() => pick(f)}></button>{/each}
</div>
{#if onToggleFav && validHex(value)}
  <div class="colorrow">
    <button onclick={() => onToggleFav(value.toLowerCase())}>{favs.includes(value.toLowerCase()) ? '★' : '☆'} {$t('mat.fav')}</button>
  </div>
{/if}

<style>
  .colorrow { display: flex; gap: 4px; align-items: center; margin-bottom: 4px; flex-wrap: wrap; }
  input[type='color'] { width: 40px; height: 30px; padding: 2px; }
  input.hex { width: 90px; font-family: monospace; }
  .sw { display: inline-block; width: 18px; height: 18px; border: 1px solid var(--sw-border); border-radius: 3px; padding: 0; vertical-align: middle; cursor: pointer; margin: 1px; }
  .sw.fav { outline: 1px dashed var(--muted); outline-offset: 1px; }
</style>
