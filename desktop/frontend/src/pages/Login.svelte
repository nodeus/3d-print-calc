<script lang="ts">
  import { wails } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user } from '../lib/session';

  let login = $state('admin');
  let password = $state('');
  let error = $state('');
  let pending = $state(false);

  async function submit() {
    if (pending) return; // защита от даблкликов: повторный argon2 вешает слабые машины
    pending = true;
    error = '';
    try {
      const timeout = new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error($t('auth.timeout'))), 20000),
      );
      user.set(await Promise.race([wails.login(login, password), timeout]));
    } catch (e) {
      error = String(e);
    } finally {
      pending = false;
    }
  }
</script>

<div class="login">
  <h2>{$t('auth.title')}</h2>
  <p class="note">{$t('auth.hint')}</p>
  <label>{$t('auth.login')}<input bind:value={login} disabled={pending} /></label>
  <label>{$t('auth.password')}<input type="password" bind:value={password} disabled={pending} onkeydown={(e) => e.key === 'Enter' && submit()} /></label>
  <button onclick={submit} disabled={pending}>{pending ? '…' : $t('auth.enter')}</button>
  {#if error}<p class="err">{error}</p>{/if}
</div>

<style>
  .login { max-width: 320px; margin: 60px auto; border: 1px solid #ddd; border-radius: 8px; padding: 20px; }
  label { display: flex; flex-direction: column; margin-bottom: 8px; font-size: 13px; }
  input, button { padding: 8px; font-size: 14px; }
  button:disabled { opacity: 0.6; }
  .note { color: #666; font-size: 13px; }
  .err { color: #b00; }
</style>
