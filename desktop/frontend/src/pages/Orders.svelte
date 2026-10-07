<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type Order, type OrderDetail, type Customer } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { money } from '../lib/format';

  let orders: Order[] = $state([]);
  let customers: Customer[] = $state([]);
  let detail: OrderDetail | null = $state(null);
  let newCustomerId = $state(0);
  let itemTitle = $state('');
  let itemQty = $state(1);
  let itemCost = $state(0);
  let itemPrice = $state(0);
  let payAmount = $state(0);
  let payNote = $state('');

  const statuses = ['draft', 'quoted', 'in_work', 'done', 'paid', 'cancelled'];

  async function load() {
    try {
      orders = await wails.listOrders();
      customers = await wails.listCustomers();
      if (detail) detail = await wails.orderDetail(detail.order.ID);
    } catch (e) {
      console.error(e);
    }
  }
  onMount(load);

  function custName(id: number): string {
    return customers.find((c) => c.ID === id)?.Name ?? `#${id}`;
  }

  async function create() {
    try {
      const id = await wails.createOrder(newCustomerId, 'RUB');
      await load();
      detail = await wails.orderDetail(id);
    } catch (e) {
      console.error(e);
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
      console.error(e);
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
      console.error(e);
    }
  }

  async function setStatus(s: string) {
    if (!detail) return;
    try {
      await wails.setOrderStatus(detail.order.ID, s);
      await load();
    } catch (e) {
      console.error(e);
    }
  }
</script>

<div class="cols">
  <section>
    {#if canWrite($user)}
      <div class="row">
        <select bind:value={newCustomerId}>
          <option value={0}>—</option>
          {#each customers as c}<option value={c.ID}>{c.Name}</option>{/each}
        </select>
        <button onclick={create}>{$t('ord.new')}</button>
      </div>
    {/if}
    <table>
      <thead><tr><th>ID</th><th>{$t('cust.name')}</th><th>{$t('ord.status')}</th><th>{$t('ord.total')}</th></tr></thead>
      <tbody>
        {#each orders as o}
          <tr class:sel={detail?.order.ID === o.ID} onclick={() => wails.orderDetail(o.ID).then((d) => (detail = d))}>
            <td>{o.ID}</td><td>{custName(o.CustomerID)}</td><td>{o.Status}</td><td class="n">{money(o.TotalBase / 100, o.Currency, 'ru')}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>

  {#if detail}
    <section class="card">
      <h3>{$t('ord.order')} #{detail.order.ID} · {custName(detail.order.CustomerID)}</h3>
      {#if canWrite($user)}
        <div class="row">
          <select value={detail.order.Status} onchange={(e) => setStatus(e.currentTarget.value)}>
            {#each statuses as s}<option value={s}>{s}</option>{/each}
          </select>
        </div>
      {/if}
      <table>
        <thead><tr><th>{$t('ord.item')}</th><th>{$t('ord.qty')}</th><th>{$t('calc.cost')}</th><th>{$t('calc.price')}</th></tr></thead>
        <tbody>
          {#each detail.items as it}<tr><td>{it.Title}</td><td class="n">{it.Qty}</td><td class="n">{money(it.Cost / 100, detail.order.Currency, 'ru')}</td><td class="n">{money(it.Price / 100, detail.order.Currency, 'ru')}</td></tr>{/each}
        </tbody>
      </table>
      {#if canWrite($user)}
        <div class="row">
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
        <div class="row">
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
  input, select, button { padding: 6px; font-size: 14px; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; font-variant-numeric: tabular-nums; }
  tr.sel td { background: #eef; }
  tbody tr { cursor: pointer; }
  .pay { color: #666; font-size: 13px; margin: 2px 0; }
</style>
