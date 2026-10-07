<script lang="ts">
  import { t } from '../lib/i18n';
  import { confirmState } from '../lib/confirm';

  function answer(v: boolean) {
    const req = $confirmState;
    confirmState.set(null);
    req?.resolve(v);
  }
</script>

{#if $confirmState}
  <div class="overlay no-print" role="dialog" aria-modal="true">
    <div class="card">
      <p>{$confirmState.title}</p>
      <div class="row">
        <button class="warn" onclick={() => answer(true)}>{$t('common.yes')}</button>
        <button onclick={() => answer(false)}>{$t('common.no')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.35);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .card {
    background: white;
    border-radius: 8px;
    padding: 20px 24px;
    max-width: 420px;
    font-size: 15px;
  }
  .row {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 12px;
  }
  button {
    padding: 8px 18px;
  }
  button.warn {
    border-color: #b00;
    color: #b00;
  }
</style>
