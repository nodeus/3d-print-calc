<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type Order, type OrderDetail, type Customer } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { openOrderId } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';

  let orders: Order[] = $state([]);
  let customers: Customer[] = $state([]);
  let detail: OrderDetail | null = $state(null);
  let newCustomerId = $state(0);
  let newTitle = $state('');
  let itemTitle = $state('');
  let itemQty = $state(1);
  let itemCost = $state(0);
  let itemPrice = $state(0);
  let payAmount = $state(0);
  let payNote = $state('');
  let error = $state('');
  // Правка шапки
  let editOpen = $state(false);
  let editCustomerId = $state(0);
  let editTitle = $state('');
  let editMarginProfit = $state(0);
  let editMarginDev = $state(0);
  let editVat = $state(0);

  const statuses = ['draft', 'quoted', 'in_work', 'done', 'paid', 'cancelled'];

  function orderNo(o: { Number: string; ID: number }): string {
    return o.Number || `#${o.ID}`;
  }

  function orderTitle(o: { Title: string; Number: string; ID: number }): string {
    return o.Title || orderNo(o);
  }

  async function load() {
    try {
      orders = (await wails.listOrders()) ?? [];
      customers = (await wails.listCustomers()) ?? [];
      if (detail) detail = await wails.orderDetail(detail.order.ID);
    } catch (e) {
      error = String(e);
    }
  }

  async function openDetail(id: number) {
    try {
      detail = await wails.orderDetail(id);
    } catch (e) {
      error = String(e);
    }
  }

  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    // Переход из долгов: открыть заказ.
    const unsub = openOrderId.subscribe((id) => {
      if (id) {
        openOrderId.set(null);
        openDetail(id);
      }
    });
    return () => {
      unsub();
      registerPdfExporter(null);
    };
  });

  function custName(id: number): string {
    return customers.find((c) => c.ID === id)?.Name ?? `#${id}`;
  }

  async function create() {
    error = '';
    try {
      // Дефолт названия: «Клиент — ДД.ММ.ГГГГ».
      const now = new Date();
      const today = `${String(now.getDate()).padStart(2, '0')}.${String(now.getMonth() + 1).padStart(2, '0')}.${now.getFullYear()}`;
      const title = newTitle.trim() || (newCustomerId ? `${custName(newCustomerId)} — ${today}` : today);
      const id = await wails.createOrder(newCustomerId, 'RUB', title);
      newTitle = '';
      await load();
      detail = await wails.orderDetail(id);
    } catch (e) {
      error = String(e);
    }
  }

  function startEditHead() {
    if (!detail) return;
    editOpen = true;
    editCustomerId = detail.order.CustomerID;
    editTitle = detail.order.Title;
    editMarginProfit = detail.order.MarginProfit;
    editMarginDev = detail.order.MarginDev;
    editVat = detail.order.VAT;
  }

  async function saveHead() {
    if (!detail) return;
    error = '';
    try {
      await wails.updateOrder({
        ...detail.order,
        CustomerID: editCustomerId,
        Title: editTitle,
        MarginProfit: editMarginProfit,
        MarginDev: editMarginDev,
        VAT: editVat,
      });
      editOpen = false;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function removeOrder() {
    if (!detail) return;
    error = '';
    if (!confirm(`${orderNo(detail.order)}?`)) return;
    try {
      await wails.deleteOrder(detail.order.ID);
      detail = null;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function addItem() {
    if (!detail || !itemTitle.trim()) return;
    try {
      await wails.addOrderItem({
        orderId: detail.order.ID, jobId: 0, title: itemTitle.trim(),
        qty: itemQty, cost: Math.round(itemCost * 100), price: Math.round(itemPrice * 100),
      });
      itemTitle = '';
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function addPay() {
    if (!detail || payAmount <= 0) return;
    try {
      await wails.addPayment(detail.order.ID, Math.round(payAmount * 100), payNote);
      payAmount = 0;
      payNote = '';
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function setStatus(s: string) {
    if (!detail) return;
    try {
      await wails.setOrderStatus(detail.order.ID, s);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function toPDF() {
    if (detail) {
      const cur = detail.order.Currency;
      await downloadPDF(`${$t('ord.number')}: ${orderNo(detail.order)}`, [
        {
          Title: '',
          Headers: [$t('ord.item'), $t('ord.qty'), $t('calc.cost'), $t('calc.price')],
          Rows: detail.items.map((it) => [it.Title, String(it.Qty), money(it.Cost / 100, cur, 'ru'), money(it.Price / 100, cur, 'ru')]),
        },
      ]);
    } else {
      await downloadPDF($t('nav.orders'), [
        {
          Title: '',
          Headers: [$t('ord.number'), $t('ord.title'), $t('cust.name'), $t('ord.status'), $t('ord.total')],
          Rows: orders.map((o) => [orderNo(o), orderTitle(o), custName(o.CustomerID), $t('st.' + o.Status), money(o.TotalBase / 100, o.Currency, 'ru')]),
        },
      ]);
    }
  }
</script>

{#if error}<p class="err">{error}</p>{/if}
<div class="cols">
  <section>
    {#if canWrite($user)}
      <div class="row no-print">
        <select bind:value={newCustomerId}>
          <option value={0}>—</option>
          {#each customers as c}<option value={c.ID}>{c.Name}</option>{/each}
        </select>
        <input placeholder={$t('ord.title')} bind:value={newTitle} />
        <button onclick={create}>{$t('ord.new')}</button>
      </div>
    {/if}
    <table>
      <thead><tr><th>{$t('ord.number')}</th><th>{$t('ord.title')}</th><th>{$t('cust.name')}</th><th>{$t('ord.status')}</th><th class="n">{$t('ord.total')}</th></tr></thead>
      <tbody>
        {#each orders as o}
          <tr class:sel={detail?.order.ID === o.ID} onclick={() => openDetail(o.ID)}>
            <td>{orderNo(o)}</td><td>{orderTitle(o)}</td><td>{custName(o.CustomerID)}</td><td>{$t('st.' + o.Status)}</td><td class="n">{money(o.TotalBase / 100, o.Currency, 'ru')}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>

  {#if detail}
    <section class="card">
      <h3>{$t('ord.number')}: {orderNo(detail.order)} · {orderTitle(detail.order)}</h3>
      {#if canWrite($user)}
        <div class="row no-print">
          <select value={detail.order.Status} onchange={(e) => setStatus(e.currentTarget.value)}>
            {#each statuses as s}<option value={s}>{$t('st.' + s)}</option>{/each}
          </select>
          <button onclick={startEditHead}>✎</button>
          <button onclick={removeOrder}>✕</button>
        </div>
        {#if editOpen}
          <div class="row no-print">
            <label>{$t('cust.name')}
              <select bind:value={editCustomerId}>
                <option value={0}>—</option>
                {#each customers as c}<option value={c.ID}>{c.Name}</option>{/each}
              </select>
            </label>
            <label>{$t('ord.title')}<input bind:value={editTitle} /></label>
            <label>{$t('calc.margin_profit')}<input type="number" min="0" step="0.01" bind:value={editMarginProfit} /></label>
            <label>{$t('calc.margin_dev')}<input type="number" min="0" step="0.01" bind:value={editMarginDev} /></label>
            <label>{$t('calc.vat')}<input type="number" min="0" step="0.01" bind:value={editVat} /></label>
            <button onclick={saveHead}>{$t('common.save')}</button>
            <button onclick={() => (editOpen = false)}>{$t('common.cancel')}</button>
          </div>
        {/if}
      {/if}
      <table>
        <thead><tr><th>{$t('ord.item')}</th><th class="n">{$t('ord.qty')}</th><th class="n">{$t('calc.cost')}</th><th class="n">{$t('calc.price')}</th></tr></thead>
        <tbody>
          {#each detail.items as it}<tr><td>{it.Title}</td><td class="n">{it.Qty}</td><td class="n">{money(it.Cost / 100, detail.order.Currency, 'ru')}</td><td class="n">{money(it.Price / 100, detail.order.Currency, 'ru')}</td></tr>{/each}
        </tbody>
      </table>
      {#if canWrite($user)}
        <div class="row no-print">
          <input placeholder={$t('ord.item')} bind:value={itemTitle} />
          <input type="number" min="0" step="0.5" bind:value={itemQty} />
          <input type="number" min="0" placeholder={$t('calc.cost')} bind:value={itemCost} />
          <input type="number" min="0" placeholder={$t('calc.price')} bind:value={itemPrice} />
          <button onclick={addItem}>+</button>
        </div>
      {/if}
      <p>{$t('ord.paid')}: {money(detail.paid / 100, detail.order.Currency, 'ru')} · {$t('ord.balance')}: <b>{money(detail.balance / 100, detail.order.Currency, 'ru')}</b></p>
      {#each detail.payments as p}<p class="pay">+{money(p.Amount / 100, detail.order.Currency, 'ru')} · {p.PaidAt} {p.Note}</p>{/each}
      {#if canWrite($user)}
        <div class="row no-print">
          <input type="number" min="0" placeholder={$t('ord.pay')} bind:value={payAmount} />
          <input placeholder={$t('ord.note')} bind:value={payNote} />
          <button onclick={addPay}>{$t('ord.pay_add')}</button>
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .cols { display: grid; grid-template-columns: 1fr 1.2fr; gap: 12px; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; }
  .row { display: flex; gap: 6px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 13px; }
  tr.sel td { background: #eef; }
  tbody tr { cursor: pointer; }
  .pay { color: #666; font-size: 13px; margin: 2px 0; }
  .err { color: #b00; }
</style>
