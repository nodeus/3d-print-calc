<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type OrderRow, type OrderDetail, type Customer, type OrderItem } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { openOrderId, customerFilter } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';

  let rows: OrderRow[] = $state([]);
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
  let payItemId = $state(0);
  let error = $state('');
  // Правка шапки
  let editOpen = $state(false);
  let editCustomerId = $state(0);
  let editTitle = $state('');
  let editDiscountPct = $state(0);
  let editDiscountFixed = $state(0);

  const statuses = ['draft', 'quoted', 'in_work', 'done', 'paid', 'cancelled'];

  function orderNo(o: { Number: string; ID: number }): string {
    return o.Number || `#${o.ID}`;
  }

  function orderTitle(o: { Title: string; Number: string; ID: number }): string {
    return o.Title || orderNo(o);
  }

  async function load() {
    try {
      rows = (await wails.listOrderRows()) ?? [];
      customers = (await wails.listCustomers()) ?? [];
      if (detail) detail = await wails.orderDetail(detail.order.ID);
    } catch (e) {
      error = String(e);
    }
  }

  function rowOf(id: number): OrderRow | undefined {
    return rows.find((r) => r.order.ID === id);
  }

  function payStateLabel(s: string): string {
    return $t('pay.' + s);
  }

  // Оплачено по позиции (сумма платежей с item_id).
  function itemPaid(itemId: number): number {
    if (!detail) return 0;
    return detail.payments.filter((p) => p.OrderItem === itemId).reduce((a, p) => a + p.Amount, 0);
  }

  function itemState(it: OrderItem): string {
    if (it.PaidManual) return 'paid';
    const total = Math.round(it.Price * it.Qty);
    const paid = itemPaid(it.ID);
    if (paid >= total) return 'paid';
    if (paid <= 0) return 'unpaid';
    return 'partial';
  }

  async function togglePaid(it: OrderItem) {
    error = '';
    try {
      await wails.toggleItemPaid(it.ID, !it.PaidManual);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  function itemTitleOf(id: number): string {
    if (!detail || !id) return $t('ord.whole_order');
    return `#${detail.items.findIndex((it) => it.ID === id) + 1} ${detail.items.find((it) => it.ID === id)?.Title ?? ''}`;
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
    // Переход из долгов: открыть заказ. Фильтр по заказчику — из вкладки заказчиков.
    const unsub1 = openOrderId.subscribe((id) => {
      if (id) {
        openOrderId.set(null);
        openDetail(id);
      }
    });
    const unsub2 = customerFilter.subscribe(() => {});
    return () => {
      unsub1();
      unsub2();
      registerPdfExporter(null);
    };
  });

  let visibleRows = $derived(
    $customerFilter ? rows.filter((r) => r.order.CustomerID === $customerFilter) : rows,
  );

  function clearCustomerFilter() {
    customerFilter.set(0);
  }

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
    editDiscountPct = detail.order.DiscountPct;
    editDiscountFixed = detail.order.DiscountFixed / 100;
  }

  async function saveHead() {
    if (!detail) return;
    error = '';
    try {
      await wails.updateOrder({
        ...detail.order,
        CustomerID: editCustomerId,
        Title: editTitle,
        DiscountPct: editDiscountPct,
        DiscountFixed: Math.round(editDiscountFixed * 100),
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
    if (!(await askConfirm(`${$t("common.delete_q")} «${orderNo(detail.order)}»?`))) return;
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
      await wails.addPayment(detail.order.ID, payItemId, Math.round(payAmount * 100), payNote);
      payAmount = 0;
      payNote = '';
      payItemId = 0;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function removeItem(id: number, title: string) {
    if (!detail) return;
    error = '';
    if (!(await askConfirm(`${$t("common.delete_q")} «${title}»?`))) return;
    try {
      await wails.deleteOrderItem(id);
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
          Title: orderTitle(detail.order),
          Headers: ['№', $t('ord.item'), $t('ord.qty'), $t('calc.price'), $t('ord.paid'), $t('ord.pay_state')],
          Rows: detail.items.map((it, i) => [
            String(i + 1), it.Title, String(it.Qty),
            money(Math.round(it.Price * it.Qty) / 100, cur, 'ru'),
            money(itemPaid(it.ID) / 100, cur, 'ru'), payStateLabel(itemState(it)),
          ]),
        },
        {
          Title: $t('ord.pays'),
          Headers: ['#', $t('ord.item'), $t('ord.pay'), $t('hist.start')],
          Rows: detail.payments.map((p, i) => [
            String(i + 1), itemTitleOf(p.OrderItem),
            money(p.Amount / 100, cur, 'ru'), p.PaidAt,
          ]),
        },
      ]);
    } else {
      await downloadPDF($t('nav.orders'), [
        {
          Title: '',
          Headers: [$t('ord.number'), $t('ord.title'), $t('cust.name'), $t('ord.status'), $t('ord.total'), $t('ord.paid'), $t('ord.balance'), $t('ord.pay_state')],
          Rows: visibleRows.map((r) => [orderNo(r.order), orderTitle(r.order), custName(r.order.CustomerID), $t('st.' + r.order.Status), money(r.order.TotalBase / 100, r.order.Currency, 'ru'), money(r.paid / 100, r.order.Currency, 'ru'), money(r.balance / 100, r.order.Currency, 'ru'), payStateLabel(r.payState)]),
        },
      ]);
    }
  }
</script>

{#if error}<p class="err">{error}</p>{/if}
{#if $customerFilter}
  <p class="filterchip no-print">{$t('ord.filtered_by')}: {custName($customerFilter)} <button onclick={clearCustomerFilter}>✕</button></p>
{/if}
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
      <thead><tr><th>{$t('ord.number')}</th><th>{$t('ord.title')}</th><th>{$t('cust.name')}</th><th>{$t('ord.status')}</th><th class="n">{$t('ord.total')}</th><th class="n">{$t('ord.paid')}</th><th class="n">{$t('ord.balance')}</th><th>{$t('ord.pay_state')}</th></tr></thead>
      <tbody>
        {#each visibleRows as r}
          <tr class:sel={detail?.order.ID === r.order.ID} onclick={() => openDetail(r.order.ID)}>
            <td>{orderNo(r.order)}</td><td>{orderTitle(r.order)}</td><td>{custName(r.order.CustomerID)}</td><td>{$t('st.' + r.order.Status)}</td><td class="n">{money(r.order.TotalBase / 100, r.order.Currency, 'ru')}</td><td class="n">{money(r.paid / 100, r.order.Currency, 'ru')}</td><td class="n">{money(r.balance / 100, r.order.Currency, 'ru')}</td><td>{payStateLabel(r.payState)}</td>
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
            <label>{$t('ord.title')}<input bind:value={editTitle} placeholder={$t('ord.title')} /></label>
            <label>{$t('ord.discount_pct')}<input placeholder="10" type="number" min="0" max="100" step="0.1" bind:value={editDiscountPct} /></label>
            <label>{$t('ord.discount_fixed')}<input placeholder="500" type="number" min="0" bind:value={editDiscountFixed} /></label>
            <button onclick={saveHead}>{$t('common.save')}</button>
            <button onclick={() => (editOpen = false)}>{$t('common.cancel')}</button>
          </div>
        {/if}
      {/if}
      {#if detail.order.DiscountPct || detail.order.DiscountFixed}
        <p>{$t('ord.discount')}: {detail.order.DiscountPct}% + {money(detail.order.DiscountFixed / 100, detail.order.Currency, 'ru')}</p>
      {/if}
      <table>
        <thead><tr><th>№</th><th>{$t('ord.item')}</th><th class="n">{$t('ord.qty')}</th><th class="n">{$t('calc.cost')}</th><th class="n">{$t('calc.price')}</th><th class="n">{$t('ord.paid')}</th><th>{$t('ord.pay_state')}</th><th class="no-print"></th></tr></thead>
        <tbody>
          {#each detail.items as it, i}<tr><td>{i + 1}</td><td>{it.Title}</td><td class="n">{it.Qty}</td><td class="n">{money(it.Cost / 100, detail.order.Currency, 'ru')}</td><td class="n">{money(it.Price / 100, detail.order.Currency, 'ru')}</td><td class="n">{money(itemPaid(it.ID) / 100, detail.order.Currency, 'ru')}</td><td>{payStateLabel(itemState(it))}{#if it.PaidManual} ✓{/if}</td><td class="no-print">{#if canWrite($user)}<button onclick={() => togglePaid(it)} title={$t(it.PaidManual ? 'ord.unmark_paid' : 'ord.mark_paid')}>{it.PaidManual ? '✔' : '○'}</button> <button onclick={() => removeItem(it.ID, it.Title)}>✕</button>{/if}</td></tr>{/each}
        </tbody>
      </table>
      {#if canWrite($user)}
        <div class="row no-print">
          <input placeholder={$t('ord.item_ph')} bind:value={itemTitle} />
          <input placeholder="1" type="number" min="0" step="0.5" bind:value={itemQty} />
          <input type="number" min="0" placeholder={$t('calc.cost')} bind:value={itemCost} />
          <input type="number" min="0" placeholder={$t('calc.price')} bind:value={itemPrice} />
          <button onclick={addItem}>+</button>
        </div>
      {/if}
      <p>{$t('ord.paid')}: {money(detail.paid / 100, detail.order.Currency, 'ru')} · {$t('ord.balance')}: <b>{money(detail.balance / 100, detail.order.Currency, 'ru')}</b></p>
      {#each detail.payments as p}<p class="pay">+{money(p.Amount / 100, detail.order.Currency, 'ru')} · {p.PaidAt} · {p.OrderItem ? itemTitleOf(p.OrderItem) : $t('ord.whole_order')} {p.Note}</p>{/each}
      {#if canWrite($user)}
        <div class="row no-print">
          <select bind:value={payItemId}>
            <option value={0}>{$t('ord.whole_order')}</option>
            {#each detail.items as it, i}<option value={it.ID}>#{i + 1} {it.Title}</option>{/each}
          </select>
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
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; }
  .row { display: flex; gap: 6px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 13px; }
  tr.sel td { background: var(--sel-bg); }
  tbody tr { cursor: pointer; }
  .filterchip { background: var(--sel-bg); border-radius: 4px; padding: 4px 8px; display: inline-block; }
  .pay { color: var(--muted); font-size: 13px; margin: 2px 0; }
  .err { color: var(--err); }
</style>
