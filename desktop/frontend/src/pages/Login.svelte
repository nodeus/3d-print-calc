<script lang="ts">
  import { wails } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user } from '../lib/session';

  let login = $state('admin');
  let password = $state('');
  let error = $state('');

  async function submit() {
    error = '';
    try {
      user.set(await wails.login(login, password));
    } catch (e) {
      error = String(e);
    }
  }
</script>

<div class="login">
  <h2>{$t('auth.title')}</h2>
  <p class="note">{$t('auth.hint')}</p>
  <label>{$t('auth.login')}<input bind:value={login} /></label>
  <label>{$t('auth.password')}<input type="password" bind:value={password} onkeydown={(e) => e.key === 'Enter' && submit()} /></label>
  <button onclick={submit}>{$t('auth.enter')}</button>
  {#if error}<p class="err">{error}</p>{/if}
</div>

<style>
  .login { max-width: 320px; margin: 60px auto; border: 1px solid #ddd; border-radius: 8px; padding: 20px; }
  label { display: flex; flex-direction: column; margin-bottom: 8px; font-size: 13px; }
  input, button { padding: 8px; font-size: 14px; }
  .note { color: #666; font-size: 13px; }
  .err { color: #b00; }
</style>
