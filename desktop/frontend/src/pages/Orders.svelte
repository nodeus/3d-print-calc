<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { get } from 'svelte/store';
  import { wails, type OrderRow, type OrderDetail, type Customer, type OrderItem } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { openOrderId, customerFilter } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings, saveMySettingsPatch } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';

  let rows: OrderRow[] = $state([]);
  let sortKey = $state('created');
  let sortDir: 1 | -1 = $state(-1);

  function toggleSort(key: string) {
    if (sortKey === key) sortDir = sortDir === 1 ? -1 : 1;
    else { sortKey = key; sortDir = 1; }
  }

  function arrow(key: string): string {
    return sortKey === key ? (sortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  function sortVal(r: OrderRow): string | number {
    switch (sortKey) {
      case 'number': return orderNo(r.order).toLowerCase();
      case 'created': return r.order.CreatedAt || '';
      case 'title': return orderTitle(r.order).toLowerCase();
      case 'customer': return custName(r.order.CustomerID).toLowerCase();
      case 'status': return r.order.Status;
      case 'total': return r.order.TotalBase;
      case 'paid': return r.paid;
      case 'balance': return r.balance;
      case 'paystate': return r.payState;
      default: return r.order.CreatedAt || '';
    }
  }

  const orderCols = [
    { key: 'number', label: 'ord.number' }, { key: 'created', label: 'ord.created' },
    { key: 'title', label: 'ord.title' }, { key: 'customer', label: 'cust.name' },
    { key: 'status', label: 'ord.status' }, { key: 'total', label: 'ord.total', num: true },
    { key: 'paid', label: 'ord.paid', num: true }, { key: 'balance', label: 'ord.balance', num: true },
    { key: 'paystate', label: 'ord.pay_state' },
  ];
  const orderKeys = orderCols.map((c) => c.key);
  const orderNum = (k: string) => orderCols.some((c) => c.key === k && c.num);
  const orderLabel = (k: string) => orderCols.find((c) => c.key === k)?.label ?? k;

  let listOrd = $derived(orderFor($mySettings.colOrder, 'orders', orderKeys));

  function orderText(r: OrderRow, k: string): string {
    switch (k) {
      case 'number': return orderNo(r.order);
      case 'created': return r.order.CreatedAt;
      case 'title': return orderTitle(r.order);
      case 'customer': return custName(r.order.CustomerID);
      case 'status': return $t('st.' + r.order.Status);
      case 'total': return money(r.order.TotalBase / 100, r.order.Currency, 'ru');
      case 'paid': return money(r.paid / 100, r.order.Currency, 'ru');
      case 'balance': return money(r.balance / 100, r.order.Currency, 'ru');
      case 'paystate': return payStateLabel(r.payState);
      default: return '';
    }
  }

  // Сортировка позиций в деталях.
  let itemSortKey = $state('n');
  let itemSortDir: 1 | -1 = $state(1);

  function toggleItemSort(key: string) {
    if (itemSortKey === key) itemSortDir = itemSortDir === 1 ? -1 : 1;
    else { itemSortKey = key; itemSortDir = 1; }
  }

  function itemArrow(key: string): string {
    return itemSortKey === key ? (itemSortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  const itemCols = [
    { key: 'title', label: 'ord.item' }, { key: 'qty', label: 'ord.qty', num: true },
    { key: 'cost', label: 'calc.cost', num: true }, { key: 'price', label: 'calc.price', num: true },
    { key: 'discount', label: 'ord.discount_col', num: true }, { key: 'itotal', label: 'ord.item_total', num: true },
    { key: 'paid', label: 'ord.paid', num: true }, { key: 'state', label: 'ord.pay_state' },
  ];
  const itemKeys = itemCols.map((c) => c.key);
  const itemNum = (k: string) => itemCols.some((c) => c.key === k && c.num);
  const itemLabel = (k: string) => itemCols.find((c) => c.key === k)?.label ?? k;

  let itemOrd = $derived(orderFor($mySettings.colOrder, 'orderItems', itemKeys));

  function itemText(it: OrderItem, k: string): string {
    const cur = detail?.order.Currency ?? 'RUB';
    switch (k) {
      case 'title': return it.Title;
      case 'qty': return String(it.Qty);
      case 'cost': return money(itemCostSum(it) / 100, cur, 'ru');
      case 'price': return money(itemPriceSum(it) / 100, cur, 'ru');
      case 'discount': return money(itemDiscount(it) / 100, cur, 'ru');
      case 'itotal': return money(itemTotal(it) / 100, cur, 'ru');
      case 'paid': return money(itemPaid(it.ID) / 100, cur, 'ru');
      case 'state': return payStateLabel(itemState(it)) + (it.PaidManual ? ' ✓' : '');
      default: return '';
    }
  }

  // Drag-перестановка за грип (обе таблицы).
  let dragKey: string | null = $state(null);
  let dragTable = $state('');
  let dropX: number | null = $state(null);
  let wrapEl: HTMLDivElement | null = $state(null);
  let itemWrapEl: HTMLDivElement | null = $state(null);

  async function dropCols(table: string, next: string[]) {
    error = '';
    try {
      await saveTableOrder(table, next);
    } catch (e) {
      error = String(e);
    }
  }

  function gripDown(e: PointerEvent, table: string, wrap: HTMLDivElement | null, k: string, ord: string[]) {
    if (!canWrite(get(user))) return;
    dragTable = table;
    startColDrag(e, wrap, k, ord, (dk, x) => { dragKey = dk; dropX = x; }, (next) => dropCols(table, next));
  }

  function shownItems(items: OrderItem[]): OrderItem[] {
    if (!items) return [];
    const val = (it: OrderItem): string | number => {
      switch (itemSortKey) {
        case 'title': return it.Title.toLowerCase();
        case 'qty': return it.Qty;
        case 'cost': return it.Cost;
        case 'price': return it.Price;
        case 'discount': return itemDiscount(it);
        case 'itotal': return itemTotal(it);
        case 'paid': return itemPaid(it.ID);
        case 'state': return itemState(it);
        default: return it.ID;
      }
    };
    return [...items].sort((a, b) => {
      const va = val(a);
      const vb = val(b);
      const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
      return cmp * itemSortDir;
    });
  }
  let customers: Customer[] = $state([]);
  let detail: OrderDetail | null = $state(null);
  let newCustomerId = $state(0);
  let newTitle = $state('');
  let itemTitle = $state('');
  let itemQty: number | null = $state(null);
  let itemCost: number | null = $state(null);
  let itemPrice: number | null = $state(null);
  let payAmount: number | null = $state(null);
  let payNote = $state('');
  let payItemId = $state(0);
  let error = $state('');
  // Правка шапки
  let editOpen = $state(false);
  let editCustomerId = $state(0);
  let editTitle = $state('');
  let editDiscountPct: number | null = $state(null);
  let editDiscountFixed: number | null = $state(null);

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
    if (total <= 0 && paid <= 0) return 'unknown';
    if (paid >= total) return 'paid';
    if (paid <= 0) return 'unpaid';
    return 'partial';
  }

  // Черновик позиции: предпросмотр строки до добавления.
  let draftQty = $derived(itemQty ?? 1);
  let draftPrice = $derived(Math.round((itemPrice ?? 0) * draftQty));
  let draftShare = $derived.by(() => {
    if (!detail) return 0;
    const pct = detail.order.DiscountPct || 0;
    const fixed = detail.order.DiscountFixed || 0;
    if (!pct && !fixed) return 0;
    const sub = itemsSubtotal() + draftPrice;
    let s = Math.round((draftPrice * pct) / 100);
    if (fixed && sub > 0) s += Math.round((fixed * draftPrice) / sub);
    return s;
  });
  let draftTotal = $derived(draftPrice - draftShare);

  // Разбор ячейки для title-подсказки: откуда взялось число.
  function itemBreakdown(it: OrderItem, k: string): string | undefined {
    const cur = detail?.order.Currency ?? 'RUB';
    const unit = (v: number) => money(v / 100, cur, 'ru');
    const line = itemPriceSum(it);
    if (k === 'price') return `${unit(it.Price)} × ${it.Qty} = ${unit(line)}`;
    if (k === 'cost') return `${unit(it.Cost)} × ${it.Qty} = ${unit(itemCostSum(it))}`;
    if (!detail) return undefined;
    if (k === 'discount') {
      const pct = detail.order.DiscountPct || 0;
      const fixed = detail.order.DiscountFixed || 0;
      if (!pct && !fixed) return undefined;
      const sub = itemsSubtotal();
      const parts = [`${pct}% от ${unit(line)}`];
      if (fixed && sub > 0) parts.push(`фикс. ${unit(fixed)} × ${line}/${sub}`);
      return `${parts.join(' + ')} = ${unit(itemDiscount(it))}`;
    }
    if (k === 'itotal') return `${unit(line)} − ${unit(itemDiscount(it))} = ${unit(itemTotal(it))}`;
    return undefined;
  }

  // Построчные суммы позиции (итог заказа — их сумма, см. RecalcTotal).
  function itemCostSum(it: OrderItem): number {
    return Math.round(it.Cost * it.Qty);
  }

  function itemPriceSum(it: OrderItem): number {
    return Math.round(it.Price * it.Qty);
  }

  function itemsSubtotal(): number {
    if (!detail) return 0;
    return detail.items.reduce((s, it) => s + itemPriceSum(it), 0);
  }

  // Доля скидки заказа на позицию: сначала % от строки, затем фикс.
  // распределяется пропорционально строкам (при пустой сумме — нули).
  function itemDiscount(it: OrderItem): number {
    if (!detail) return 0;
    const line = itemPriceSum(it);
    const sub = itemsSubtotal();
    const pct = detail.order.DiscountPct || 0;
    const fixed = detail.order.DiscountFixed || 0;
    if (!pct && !fixed) return 0;
    let share = Math.round((line * pct) / 100);
    if (fixed && sub > 0) share += Math.round((fixed * line) / sub);
    return share;
  }

  function itemTotal(it: OrderItem): number {
    return itemPriceSum(it) - itemDiscount(it);
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

  let visibleRows = $derived.by(() => {
    const base = $customerFilter ? rows.filter((r) => r.order.CustomerID === $customerFilter) : rows;
    return [...base].sort((a, b) => {
      const va = sortVal(a);
      const vb = sortVal(b);
      const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
      return cmp * sortDir;
    });
  });

  // Пагинация списка заказов; размер — из настроек пользователя (дефолт 20).
  let page = $state(0);
  let perPage = $derived($mySettings.ordersPerPage > 0 ? $mySettings.ordersPerPage : 20);
  let pages = $derived(Math.max(1, Math.ceil(visibleRows.length / perPage)));
  let paged = $derived(visibleRows.slice(page * perPage, page * perPage + perPage));

  $effect(() => {
    void visibleRows.length;
    void perPage;
    untrack(() => {
      if (page > pages - 1) page = pages - 1;
      if (page < 0) page = 0;
    });
  });

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
        DiscountPct: editDiscountPct ?? 0,
        DiscountFixed: Math.round((editDiscountFixed ?? 0) * 100),
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
        qty: itemQty ?? 1, cost: Math.round((itemCost ?? 0) * 100), price: Math.round((itemPrice ?? 0) * 100),
      });
      itemTitle = '';
      itemQty = null;
      itemCost = null;
      itemPrice = null;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function addPay() {
    if (!detail || (payAmount ?? 0) <= 0) return;
    try {
      await wails.addPayment(detail.order.ID, payItemId, Math.round((payAmount ?? 0) * 100), payNote);
      payAmount = null;
      payNote = '';
      payItemId = 0;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  // Подставить оплату позиции (остаток с учётом скидки) в форму оплаты.
  function fillItemPay(it: OrderItem) {
    payItemId = it.ID;
    payAmount = Math.max(0, Math.round(itemTotal(it) - itemPaid(it.ID)) / 100);
  }

  // Подставить весь заказ (итог со скидкой) как оплату всего заказа.
  function fillWholePay() {
    if (!detail) return;
    payItemId = 0;
    payAmount = Math.round(detail.order.TotalBase) / 100;
  }

  async function removeItem(id: number, title: string) {
    if (!detail) return;
    error = '';
    if (!(await askConfirm(`${$t("common.delete_q")} «${title}»? ${$t("ord.del_item_note")}`))) return;
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
      await downloadPDF(`${$t('ord.number')}: ${orderNo(detail.order)} · ${detail.order.CreatedAt}`, [
        {
          Title: orderTitle(detail.order),
          Headers: ['№', ...itemOrd.map((k) => $t(itemLabel(k)))],
          Rows: shownItems(detail.items).map((it, i) => [
            String(i + 1), ...itemOrd.map((k) => itemText(it, k)),
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
          Headers: listOrd.map((k) => $t(orderLabel(k))),
          Rows: visibleRows.map((r) => listOrd.map((k) => orderText(r, k))),
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
    <div class="twrap" bind:this={wrapEl}>
    {#if dragTable === 'orders' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
    <table class="tw" class:dragging={dragTable === 'orders' && dragKey != null}>
      <thead><tr>
        {#each listOrd as k}
          <th class:n={orderNum(k)} style={thWidth('orders', k)} data-col={k} class:dragsrc={dragTable === 'orders' && dragKey === k}>
            {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'orders', wrapEl, k, listOrd)}>⋮⋮</span>{/if}
            <button class="th" onclick={() => toggleSort(k)}>{$t(orderLabel(k))}{arrow(k)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'orders', k)} ondblclick={() => resetColWidth('orders', k)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>
        {#each paged as r}
          <tr class:sel={detail?.order.ID === r.order.ID} onclick={() => openDetail(r.order.ID)}>
            {#each listOrd as k}<td class:n={orderNum(k)}>{orderText(r, k)}</td>{/each}
          </tr>
        {/each}
      </tbody>
    </table>
    </div>
    <div class="row no-print" style="margin-top:6px;align-items:center;">
      <span class="count">{$t('hist.shown')}: {paged.length} / {visibleRows.length} / {rows.length}</span>
      <span class="pager">
        <button disabled={page === 0} onclick={() => page--}>‹</button>
        {page + 1} / {pages}
        <button disabled={page >= pages - 1} onclick={() => page++}>›</button>
      </span>
    </div>
  </section>

  {#if detail}
    <section class="card">
      <h3>{$t('ord.number')}: {orderNo(detail.order)} · {orderTitle(detail.order)} · {detail.order.CreatedAt}</h3>
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
      <div class="twrap" bind:this={itemWrapEl}>
      {#if dragTable === 'orderItems' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
      <table class="tw" class:dragging={dragTable === 'orderItems' && dragKey != null}>
        <thead><tr><th class="no-print" style="width:34px;">№</th>
          {#each itemOrd as k}
            <th class:n={itemNum(k)} style={thWidth('orderItems', k)} data-col={k} class:dragsrc={dragTable === 'orderItems' && dragKey === k}>
              {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'orderItems', itemWrapEl, k, itemOrd)}>⋮⋮</span>{/if}
              <button class="th" onclick={() => toggleItemSort(k)}>{$t(itemLabel(k))}{itemArrow(k)}</button>
              <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'orderItems', k)} ondblclick={() => resetColWidth('orderItems', k)}></span>
            </th>
          {/each}
          <th class="no-print"></th>
        </tr></thead>
        <tbody>
          {#each shownItems(detail.items) as it, i}<tr>
            <td>{i + 1}</td>
            {#each itemOrd as k}<td class:n={itemNum(k)} title={itemBreakdown(it, k)}>{itemText(it, k)}</td>{/each}
            <td class="no-print">{#if canWrite($user)}<button onclick={() => fillItemPay(it)} title={$t('ord.pay_item')}>₽</button> <button onclick={() => togglePaid(it)} title={$t(it.PaidManual ? 'ord.unmark_paid' : 'ord.mark_paid')}>{it.PaidManual ? '✔' : '○'}</button> <button onclick={() => removeItem(it.ID, it.Title)}>✕</button>{/if}</td>
          </tr>{/each}
        </tbody>
      </table>
      </div>
      {#if canWrite($user)}
        <div class="row no-print">
          <input placeholder={$t('ord.ph_title')} bind:value={itemTitle} />
          <input placeholder={$t('ord.ph_qty')} type="number" min="0" step="0.5" bind:value={itemQty} />
          <input type="number" min="0" placeholder={$t('ord.ph_cost')} bind:value={itemCost} />
          <input type="number" min="0" placeholder={$t('ord.ph_price')} bind:value={itemPrice} />
          <button onclick={addItem}>+</button>
        </div>
        <p class="note">{$t('ord.cost_note')}</p>
        {#if itemTitle.trim()}
          <p class="note">{$t('ord.draft_preview')}: {draftQty} × {money((itemPrice ?? 0), detail.order.Currency, 'ru')} − {money(draftShare / 100, detail.order.Currency, 'ru')} = <b>{money(draftTotal / 100, detail.order.Currency, 'ru')}</b></p>
        {/if}
        {#if (itemCost ?? 0) > 0 && (itemPrice ?? 0) <= 0}
          <p class="warnline">{$t('ord.zero_price_warn')}</p>
        {/if}
      {/if}
      <p>{$t('ord.paid')}: {money(detail.paid / 100, detail.order.Currency, 'ru')} · {$t('ord.balance')}: <b>{money(detail.balance / 100, detail.order.Currency, 'ru')}</b>
        {#if canWrite($user)} <button class="no-print" onclick={fillWholePay}>{$t('ord.pay_all')}</button>{/if}
      </p>
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
  .cols { display: grid; grid-template-columns: 1fr; gap: 12px; }
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; }
  .row { display: flex; gap: 6px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 13px; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 13px; }
  tr.sel td { background: var(--sel-bg); }
  tbody tr { cursor: pointer; }
  .filterchip { background: var(--sel-bg); border-radius: 4px; padding: 4px 8px; display: inline-block; }
  .count { color: var(--muted); font-size: 13px; }
  .pager { margin-left: auto; display: flex; gap: 4px; align-items: center; }
  .pay { color: var(--muted); font-size: 13px; margin: 2px 0; }
  .note { color: var(--muted); font-size: 13px; margin: 2px 0; }
  .warnline { color: var(--warn); font-size: 13px; margin: 2px 0; }
  .err { color: var(--err); }
</style>
