<script lang="ts">
  import { onMount } from 'svelte';
  import { wails } from '../lib/wails';
  import { t } from '../lib/i18n';
  import qrDonate from '../assets/donate-qr.png';

  const SITE = 'https://nodeus.ru';
  const GITHUB = 'https://github.com/nodeus';
  const DONATE = 'https://pay.cloudtips.ru/p/463b1251';

  let version = $state('…');
  let buildDate = $state('…');
  let counts = $state<Record<string, number>>({});

  onMount(async () => {
    try {
      const v = await wails.appVersion();
      version = v.version ?? '?';
      buildDate = v.buildDate ?? '?';
    } catch (e) {
      console.error(e);
    }
  });
</script>

<div class="card">
  <h2>{$t('app.title')} v{version} ({buildDate})</h2>
  <p>{$t('info.about')}</p>
  <h3>{$t('info.how')}</h3>
  <ol>
    <li>{$t('info.step1')}</li>
    <li>{$t('info.step2')}</li>
    <li>{$t('info.step3')}</li>
    <li>{$t('info.step4')}</li>
  </ol>
  <h3>{$t('info.moon')}</h3>
  <p>{$t('info.moon_text')}</p>
  <h3>{$t('info.admin')}</h3>
  <p>{$t('info.admin_text')}</p>
</div>

<div class="card">
  <h3>{$t('info.author')}</h3>
  <p>Ilya V Shirinkin aka nodeus</p>
  <p>
    <button class="link" onclick={() => wails.openURL(SITE)}>nodeus.ru</button>
    ·
    <button class="link" onclick={() => wails.openURL(GITHUB)}>GitHub</button>
  </p>
  <h3>{$t('info.donate')}</h3>
  <p>{$t('info.donate_text')}</p>
  <p><img src={qrDonate} alt="QR" width="160" height="160" /></p>
  <p><button class="link" onclick={() => wails.openURL(DONATE)}>{DONATE}</button></p>
</div>

<style>
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; max-width: 720px; }
  h2 { margin-top: 0; }
  li { margin-bottom: 6px; }
  button.link { background: none; border: none; color: #0645ad; cursor: pointer; padding: 0; font-size: inherit; text-decoration: underline; }
</style>
