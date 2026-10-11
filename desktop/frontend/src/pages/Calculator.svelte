<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type CalcInput, type CalcResult, type MaterialDTO, type SpoolDTO, type PrinterDTO, type Order, type JobDTO, type Customer } from '../lib/wails';
  import { t, locale } from '../lib/i18n';
  import { money } from '../lib/format';
  import { prefillJob } from '../lib/prefill';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { calcForm, factoryValues, resetCalcForm, applyCalcDefaults } from '../lib/calcform';
  import { saveMySettingsPatch } from '../lib/usersettings';
  import { askConfirm } from '../lib/confirm';
  import { notify } from '../lib/notice';
  import { splitHours, joinHours, normHM } from '../lib/hm';
  import { get } from 'svelte/store';

  let materials: MaterialDTO[] = $state([]);
  let printers: PrinterDTO[] = $state([]);
  let currency = $state('RUB');

  // Форма: числа nullable — пустое поле показывает плейсхолдер,
  // в расчёт идёт ?? дефолт (0, копии — 1).
  let matCode = $state('PETG-NIT-W4');
  let printerId = $state(0);
  let timeH: number | null = $state(3);
  let timeM: number | null = $state(54);
  let weightG: number | null = $state(134.94);
  let filamentMM: number | null = $state(null);
  let tariff: number | null = $state(7.08);
  let powerKW: number | null = $state(0.35);
  let amortPerHour: number | null = $state(5.41);
  let opRate: number | null = $state(100);
  // Время работ — парами часы+минуты (внутри везде десятичные часы).
  let prep = $state({ h: null as number | null, m: null as number | null });
  let post = $state({ h: null as number | null, m: null as number | null });
  let designerRate: number | null = $state(600);
  let model = $state({ h: null as number | null, m: null as number | null });
  let modelPrice: number | null = $state(0);
  let extraMat: number | null = $state(50);
  let failPct: number | null = $state(0);
  let marginProfitPct: number | null = $state(80);
  let marginDevPct: number | null = $state(35);
  let vatPct: number | null = $state(0);
  let roundStep: number | null = $state(0);
  let fees: number | null = $state(100);
  let maintPerHour: number | null = $state(0);
  let purgeG: number | null = $state(0);
  let filename = $state('');
  let error = $state('');
  let orders: Order[] = $state([]);
  let customers: Customer[] = $state([]);
  let customerId = $state(0);
  let orderId = $state(0);
  let orderTitle = $state('');
  let orderQty = $state(1);
  let addedInfo = $state('');

  let result = $state<CalcResult | null>(null);
  // Выбранная катушка: id; материал выводится из неё. 0 — не выбрана.
  let spoolId = $state(0);

  function spoolOf(id: number) {
    for (const m of materials) {
      const s = (m.spools ?? []).find((x) => x.id === id);
      if (s) return { mat: m, spool: s };
    }
    return null;
  }

  // Живые катушки для селекта: свои данные + подпись КОД-N.
  let selSpool = $derived(spoolOf(spoolId));
  let pricePerKg = $derived(selSpool?.spool.price ?? materials.find((m) => m.code === matCode)?.pricePerKg ?? 0);
  // Физика прутка — с выбранной катушки (фолбэк — материал, затем 1.27/1.75).
  let density = $derived(selSpool?.spool.density || materials.find((m) => m.code === matCode)?.density || 1.27);
  let diameter = $derived(selSpool?.spool.diameter || 1.75);
  // Материалы с живыми катушками (плюс уже выбранный для совместимости).
  let activeMaterials = $derived(materials.filter((m) => m.hasLiveSpool || m.code === matCode));
  // Опции селекта: живые катушки видимых материалов + текущая (если архивная).
  let spoolOptions = $derived.by(() => {
    const out: { mat: MaterialDTO; spool: SpoolDTO }[] = [];
    for (const m of activeMaterials) {
      for (const s of m.spools ?? []) {
        if (!s.archived || s.id === spoolId) out.push({ mat: m, spool: s });
      }
    }
    return out;
  });
  let historyJobs: JobDTO[] = $state([]);
  let historyId = $state(0);
  let stubWarn = $state('');

  function snapMaterial(j: JobDTO): string {
    try {
      return JSON.parse(j.CostSnapshot)?.material ?? '';
    } catch {
      return '';
    }
  }

  interface SnapInput {
    tariff?: number;
    powerKW?: number;
    amortPerHour?: number;
    opRate?: number;
    prepH?: number;
    postH?: number;
    designerRate?: number;
    modelH?: number;
    modelPrice?: number;
    extraMat?: number;
    failRate?: number;
    marginProfit?: number;
    marginDev?: number;
    fees?: number;
    maintPerHour?: number;
    purgeWasteG?: number;
    vatRate?: number;
    roundStep?: number;
  }

  function snapSpoolNo(j: JobDTO): number {
    try {
      return Number(JSON.parse(j.CostSnapshot)?.input?.spoolNo) || 0;
    } catch {
      return 0;
    }
  }

  function snapInput(j: JobDTO): SnapInput {
    try {
      return JSON.parse(j.CostSnapshot)?.input ?? {};
    } catch {
      return {};
    }
  }

  let calcReady = $state(false);
  let pendingJob: JobDTO | null = $state(null);

  // Применить работу из истории: физика (материал, вес, время, имя) — всегда;
  // ставки — только из настоящих данных: ручная работа или исправленная
  // (edited=1) импортная. Чистый импорт ставок не содержит — пользовательские
  // значения (тариф, обработка, НДС...) сохраняются.
  // Селект «Работа из истории» синхронизируется (иначе там висит прочерк).
  function applyJob(j: JobDTO) {
    if (!calcReady) {
      pendingJob = j; // loadAll ещё идёт — применим по готовности
      return;
    }
    historyId = historyJobs.some((x) => x.ID === j.ID) ? j.ID : 0;
    const code = snapMaterial(j);
    const inp = snapInput(j);
    stubWarn = '';
    if (code && materials.some((m) => m.code === code)) {
      matCode = code;
    } else if (code) {
      stubWarn = `${code}: ${$t('calc.stub_warn')}`;
    }
    // Катушка из работы: явный линк, иначе номер из снапшота, иначе первая живая.
    spoolId = 0;
    if (j.SpoolID && spoolOf(j.SpoolID)) {
      const found = spoolOf(j.SpoolID)!;
      spoolId = found.spool.id;
      matCode = found.mat.code;
    } else if (code) {
      const no = snapSpoolNo(j);
      const m = materials.find((x) => x.code === code);
      const hit = no > 0 ? (m?.spools ?? []).find((s) => s.spoolNo === no && !s.archived) : undefined;
      if (hit) {
        spoolId = hit.id;
      } else {
        pickSpool();
      }
    }
    // Принтер — только привязка (ставки берём из снапшота, они точнее текущих).
    if (j.PrinterID && printers.some((p) => p.ID === j.PrinterID)) {
      printerId = j.PrinterID;
    } else if (printers.length && !printerId) {
      selectPrinter(printers[0].ID);
    }
    const ratesReal = j.Origin === 'manual' || j.Edited === 1;
    if (ratesReal) {
      if (inp.tariff != null) tariff = inp.tariff;
      if (inp.powerKW != null) powerKW = inp.powerKW;
      if (inp.amortPerHour != null) amortPerHour = inp.amortPerHour;
      if (inp.opRate != null) opRate = inp.opRate;
      if (inp.prepH != null) prep = splitHours(inp.prepH);
      if (inp.postH != null) post = splitHours(inp.postH);
      if (inp.designerRate != null) designerRate = inp.designerRate;
      if (inp.modelH != null) model = splitHours(inp.modelH);
      if (inp.modelPrice != null) modelPrice = inp.modelPrice;
      if (inp.extraMat != null) extraMat = inp.extraMat;
      if (inp.failRate != null) failPct = inp.failRate * 100;
      if (inp.marginProfit != null) marginProfitPct = inp.marginProfit * 100;
      if (inp.marginDev != null) marginDevPct = inp.marginDev * 100;
      if (inp.fees != null) fees = inp.fees;
      if (inp.maintPerHour != null) maintPerHour = inp.maintPerHour;
      if (inp.purgeWasteG != null) purgeG = inp.purgeWasteG;
      // copies больше не используется: расчёт всегда за печать целиком.
      if (inp.vatRate != null) vatPct = inp.vatRate * 100;
      if (inp.roundStep != null) roundStep = inp.roundStep;
    }
    weightG = Math.round(j.WeightG * 100) / 100;
    filamentMM = Math.round(j.LengthM * 1000 * 100) / 100 || null;
    timeH = Math.floor(j.TotalSec / 3600);
    timeM = Math.round((j.TotalSec % 3600) / 60);
    if (timeM >= 60) {
      timeH += 1;
      timeM = 0;
    }
    filename = j.Filename;
    // Название позиции для заказа — из имени файла без расширения.
    orderTitle = j.Filename.replace(/\.gcode$/i, '');
    recalc();
    notify(`${$t('calc.applied_job')}: ${j.Filename}`);
  }

  function applyHistorySelect(id: number) {
    const j = historyJobs.find((x) => x.ID === id);
    if (j) applyJob(j);
  }

  onMount(() => {
    loadAll();
    registerPdfExporter(toPDF);
    // Подписка: prefill, пришедший при смонтированном калькуляторе.
    const unsub = prefillJob.subscribe((j) => {
      if (j) {
        prefillJob.set(null);
        applyJob(j);
      }
    });
    return () => {
      unsub();
      persist(); // добить дебаунс recalc: вкладка ничего не теряет
      registerPdfExporter(null);
    };
  });

  // pickSpool чинит выбор после загрузки справочника/работы:
  // валидный оставляем (и matCode из него), иначе первая живая текущего
  // материала, иначе первая живая вообще.
  function pickSpool() {
    const found = spoolId ? spoolOf(spoolId) : null;
    if (found) {
      matCode = found.mat.code;
      return;
    }
    spoolId = 0;
    const m = materials.find((x) => x.code === matCode);
    const live = (m?.spools ?? []).find((s) => !s.archived);
    if (live) {
      spoolId = live.id;
      return;
    }
    const any = spoolOptions[0];
    if (any) {
      spoolId = any.spool.id;
      matCode = any.mat.code;
    }
  }

  function onSpoolChange(id: number) {
    spoolId = id;
    const found = spoolOf(id);
    if (found) matCode = found.mat.code;
    recalc();
  }

  // pullForm переносит стор формы в локальные поля (монтирование, сброс).
  function pullForm() {
    const saved = get(calcForm);
    matCode = saved.matCode;
    spoolId = saved.spoolId ?? 0;
    printerId = saved.printerId;
    timeH = saved.timeH;
    timeM = saved.timeM;
    weightG = saved.weightG;
    filamentMM = saved.filamentMM;
    tariff = saved.tariff;
    powerKW = saved.powerKW;
    amortPerHour = saved.amortPerHour;
    opRate = saved.opRate;
    prep = splitHours(saved.prepH);
    post = splitHours(saved.postH);
    designerRate = saved.designerRate;
    model = splitHours(saved.modelH);
    modelPrice = saved.modelPrice;
    extraMat = saved.extraMat;
    failPct = saved.failPct;
    marginProfitPct = saved.marginProfitPct;
    marginDevPct = saved.marginDevPct;
    vatPct = saved.vatPct;
    roundStep = saved.roundStep;
    fees = saved.fees;
    maintPerHour = saved.maintPerHour;
    purgeG = saved.purgeG;
    filename = saved.filename;
    customerId = saved.customerId;
    orderId = saved.orderId;
    orderTitle = saved.orderTitle;
    orderQty = saved.orderQty;
    historyId = saved.historyId;
  }

  // Текущие значения как дефолты (без контекста заказа/истории/файла).
  // Время — десятичными часами (как в форме дефолтов).
  function currentDefaults(): Record<string, unknown> {
    return {
      matCode, printerId, timeH, timeM, weightG, filamentMM,
      tariff, powerKW, amortPerHour, opRate,
      prepH: joinHours(prep.h, prep.m), postH: joinHours(post.h, post.m),
      designerRate, modelH: joinHours(model.h, model.m), modelPrice, extraMat, failPct,
      marginProfitPct, marginDevPct, vatPct, roundStep, fees,
      maintPerHour, purgeG,
    };
  }

  async function saveAsDefaults() {
    const d = currentDefaults();
    try {
      await saveMySettingsPatch({ calcDefaults: d });
      applyCalcDefaults(d);
      notify($t('common.saved'));
    } catch (e) {
      console.error(e);
    }
  }

  async function resetToDefaults() {
    if (!(await askConfirm($t('calc.resetdef_q')))) return;
    resetCalcForm();
    pullForm();
    await recalc();
  }

  async function loadAll() {
    try {
      // Восстановить форму с прошлого визита вкладки.
      pullForm();
      materials = (await wails.materials()) ?? [];
      printers = (await wails.printers()) ?? [];
      currency = await wails.getBaseCurrency();
      orders = ((await wails.listOrders()) ?? []).filter((o) => ['draft', 'quoted', 'in_work'].includes(o.Status));
      customers = (await wails.listCustomers()) ?? [];
      if (orders.length) orderId = orders[0].ID;
      historyJobs = (await wails.jobs(0)) ?? [];
      if (materials.length && !materials.some((m) => m.code === matCode)) matCode = materials[0].code;
      pickSpool();
      if (!printers.some((p) => p.ID === printerId)) {
        printerId = 0;
        if (printers.length) selectPrinter(printers[0].ID);
      }
      if (!customers.some((c) => c.ID === customerId)) customerId = 0;
      if (!orders.some((o) => o.ID === orderId)) orderId = orders.length ? orders[0].ID : 0;
      if (!historyJobs.some((j) => j.ID === historyId)) historyId = 0;
      calcReady = true;
      // Висящий prefill: разовый из монтирования или ранний из подписки.
      const pre = pendingJob ?? get(prefillJob);
      pendingJob = null;
      if (pre) {
        prefillJob.set(null);
        applyJob(pre);
      } else {
        await recalc();
      }
    } catch (e) {
      console.error(e);
    }
  }

  function selectPrinter(id: number) {
    printerId = id;
    const p = printers.find((x) => x.ID === id);
    if (p) {
      amortPerHour = Math.round(p.amortPerHour * 100) / 100;
      powerKW = p.PowerKW;
    }
  }

  // Подпись метода амортизации человеческим языком (в селекте — не код).
  function amortMethodLabel(m: string): string {
    if (m === 'workdays') return $t('calc.amort_workdays');
    if (m === 'resource') return $t('calc.amort_resource');
    return $t('calc.amort_calendar');
  }

  // Смена принтера пользователем: ставки берутся с принтера — говорим об этом.
  // Программные вызовы selectPrinter (загрузка) молчат.
  function onPrinterChange(id: number) {
    selectPrinter(id);
    const p = printers.find((x) => x.ID === id);
    if (p) notify(`${$t('calc.printer_rates')}: ${p.Name}`);
    recalc();
  }

  async function syncWeightFromFilament() {
    if (filamentMM == null || filamentMM <= 0) return;
    try {
      weightG = Math.round((await wails.weightFromLength(density, diameter, filamentMM)) * 100) / 100;
    } catch (e) {
      error = String(e);
    }
  }

  function buildInput(): CalcInput {
    return {
      TotalSec: (timeH ?? 0) * 3600 + (timeM ?? 0) * 60,
      WeightG: weightG ?? 0,
      PricePerKg: pricePerKg,
      Tariff: tariff ?? 0,
      PowerKW: powerKW ?? 0,
      AmortPerHour: amortPerHour ?? 0,
      OpRate: opRate ?? 0,
      PrepH: joinHours(prep.h, prep.m),
      PostH: joinHours(post.h, post.m),
      DesignerRate: designerRate ?? 0,
      ModelH: joinHours(model.h, model.m),
      ModelPrice: modelPrice ?? 0,
      ExtraMat: extraMat ?? 0,
      FailRate: (failPct ?? 0) / 100,
      MarginProfit: (marginProfitPct ?? 0) / 100,
      MarginDev: (marginDevPct ?? 0) / 100,
      Fees: fees ?? 0,
      MaintPerHour: maintPerHour ?? 0,
      PurgeWasteG: purgeG ?? 0,
      Copies: 1,
      VATRate: (vatPct ?? 0) / 100,
      RoundStep: roundStep ?? 0,
    };
  }

  let timer: number | undefined;

  // persist сохраняет форму в стор вкладки (переживёт переключение).
  // Время — десятичными часами.
  function persist() {
    calcForm.set({
      matCode, spoolId, printerId, timeH, timeM, weightG, filamentMM,
      tariff, powerKW, amortPerHour, opRate,
      prepH: joinHours(prep.h, prep.m), postH: joinHours(post.h, post.m),
      designerRate, modelH: joinHours(model.h, model.m), modelPrice, extraMat, failPct,
      marginProfitPct, marginDevPct, vatPct, roundStep, fees,
      maintPerHour, purgeG, filename, customerId, orderId,
      orderTitle, orderQty, historyId,
    });
  }

  async function recalc() {
    window.clearTimeout(timer);
    timer = window.setTimeout(async () => {
      try {
        result = await wails.calc(buildInput());
        persist();
      } catch (e) {
        error = String(e);
      }
    }, 120);
  }

  function orderNo(o: Order): string {
    return o.Number || `#${o.ID}`;
  }

  function orderLabel(o: Order): string {
    const suffix = o.Title ? ` · ${o.Title}` : '';
    return `${orderNo(o)}${suffix} · ${custName(o.CustomerID)} · ${get(t)('st.' + o.Status)}`;
  }

  function custName(id: number): string {
    return customers.find((c) => c.ID === id)?.Name ?? '';
  }

  let visibleOrders = $derived(orders.filter((o) => !customerId || o.CustomerID === customerId));

  function pickCustomer(id: number) {
    customerId = id;
    const first = orders.filter((o) => !id || o.CustomerID === id)[0];
    orderId = first ? first.ID : 0;
    persist();
  }

  async function save() {
    if (!result) return;
    try {
      await wails.saveJob({
        filename,
        printerId,
        spoolId,
        status: 'completed',
        totalSec: Math.round((timeH ?? 0) * 3600 + (timeM ?? 0) * 60),
        weightG: weightG ?? 0,
        lengthM: (filamentMM ?? 0) / 1000,
        cost: result.Cost,
        price: result.PriceVAT,
        currency,
        material: matCode,
        tariff: tariff ?? 0,
        powerKW: powerKW ?? 0,
        amortPerHour: amortPerHour ?? 0,
        opRate: opRate ?? 0,
        prepH: joinHours(prep.h, prep.m),
        postH: joinHours(post.h, post.m),
        designerRate: designerRate ?? 0,
        modelH: joinHours(model.h, model.m),
        modelPrice: modelPrice ?? 0,
        extraMat: extraMat ?? 0,
        failRate: (failPct ?? 0) / 100,
        marginProfit: (marginProfitPct ?? 0) / 100,
        marginDev: (marginDevPct ?? 0) / 100,
        fees: fees ?? 0,
        maintPerHour: maintPerHour ?? 0,
        purgeWasteG: purgeG ?? 0,
        copies: 1,
        vatRate: (vatPct ?? 0) / 100,
        roundStep: roundStep ?? 0,
      });
      persist();
      notify($t('calc.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function addToOrder() {
    if (!result) return;
    error = '';
    if (!orderId || !orderTitle.trim()) {
      error = $t('calc.need_order_title');
      return;
    }
    try {
      // Цена/себестоимость — за копию: итог заказа считает price×qty.
      // (Раньше уходила цена за весь стол — при qty>1 итог кратно завышался.)
      const q = orderQty;
      await wails.addOrderItem({
        orderId, jobId: 0, title: orderTitle.trim(), qty: q,
        cost: Math.round(result.CostPerCopy * 100), price: Math.round(result.PricePerCopy * 100),
      });
      addedInfo = `${orderTitle.trim()} — ${q} × ${money(result.PricePerCopy, currency, $locale)} = ${money(result.PricePerCopy * q, currency, $locale)}`;
      notify(`${$t('ord.added')}: ${addedInfo}`);
      persist();
    } catch (e) {
      error = String(e);
    }
  }

  async function toPDF() {
    if (!result) return;
    const fmt = (v: number) => money(v, currency, $locale);
    const nv = (v: number | null) => (v == null ? '—' : String(v));
    const mat = materials.find((x) => x.code === matCode);
    const printer = printers.find((x) => x.ID === printerId);
    await downloadPDF($t('nav.calculator'), [
      {
        Title: $t('calc.params'),
        Headers: [$t('calc.param'), $t('calc.value')],
        Rows: [
          [$t('calc.material'), `${matCode} (${fmt(mat?.pricePerKg ?? 0)})`],
          [$t('calc.printer'), printer ? `${printer.Name} (${printer.Model})` : '—'],
          [$t('calc.time'), `${timeH ?? 0}h ${timeM ?? 0}m`],
          [$t('calc.weight'), `${weightG ?? 0} г`],
          [$t('calc.tariff'), nv(tariff)],
          [$t('calc.power'), nv(powerKW)],
          [$t('calc.amort'), nv(amortPerHour)],
          [$t('calc.op_rate'), `${nv(opRate)} / ${prep.h ?? 0}h ${prep.m ?? 0}m + ${post.h ?? 0}h ${post.m ?? 0}m`],
          [$t('calc.designer_rate'), `${nv(designerRate)} / ${model.h ?? 0}h ${model.m ?? 0}m + ${nv(modelPrice)}`],
          [$t('calc.extra'), nv(extraMat)],
          [$t('calc.fail'), `${nv(failPct)}%`],
          [$t('calc.margin_profit'), `${nv(marginProfitPct)}%`],
          [$t('calc.margin_dev'), `${nv(marginDevPct)}%`],
          [$t('calc.vat'), `${nv(vatPct)}%`],
        ],
      },
      {
        Title: '',
        Headers: [$t('calc.breakdown'), $t('calc.amount')],
        Rows: [
          ...rows.map(([k, v]) => [$t(k), fmt(v)]),
          [$t('calc.cost'), fmt(result.Cost)],
          [$t('calc.price'), fmt(result.Price)],
          [$t('calc.final'), fmt(result.PriceVAT)],
        ],
      },
    ]);
  }

  const rows: [string, number][] = $derived(
    result
      ? [
          ['calc.row_mat', result.MatCost],
          ['calc.row_purge', result.PurgeCost],
          ['calc.row_elec', result.ElecCost],
          ['calc.row_amort', result.Amort],
          ['calc.row_maint', result.Maint],
          ['calc.row_labor', result.Labor],
          ['calc.row_fail', result.FailAdj],
          ['calc.row_subtotal', result.Subtotal],
        ]
      : [],
  );
</script>

<div class="grid">
  <section class="card">
    {#if error}<p class="err">{error}</p>{/if}
    {#if !materials.length}<p class="warn">{$t('calc.no_materials')}</p>{/if}
    {#if materials.length && !spoolOptions.length}<p class="warn">{$t('calc.no_spools')}</p>{/if}
    {#if !printers.length}<p class="warn">{$t('calc.no_printers')}</p>{/if}
    <div class="row">
      <label>{$t('calc.job')}
        <select value={historyId} onchange={(e) => applyHistorySelect(Number(e.currentTarget.value))}>
          <option value={0}>—</option>
          {#each historyJobs as j}<option value={j.ID}>{j.Filename} ({j.StartAt})</option>{/each}
        </select>
      </label>
    </div>
    {#if stubWarn}<p class="warn">{stubWarn}</p>{/if}
    <h3 class="sec">{$t('calc.grp_material')}</h3>
    <div class="row">
      <label>{$t('calc.spool')}
        <select value={spoolId} onchange={(e) => onSpoolChange(Number(e.currentTarget.value))}>
          {#each activeMaterials as m}
            {#if (m.spools ?? []).some((s) => !s.archived || s.id === spoolId)}
              <optgroup label={m.code}>
                {#each (m.spools ?? []).filter((s) => !s.archived || s.id === spoolId) as s}
                  <option value={s.id}>{m.code}-{s.spoolNo || '?'} — {money(s.price, s.currency, $locale)}/кг{#if (s.leftG ?? s.weightLeftG) > 0}, {Math.round(s.leftG ?? s.weightLeftG)}{/if}</option>
                {/each}
              </optgroup>
            {/if}
          {/each}
        </select>
      </label>
      <label>{$t('calc.printer')}
        <select value={printerId} onchange={(e) => onPrinterChange(Number(e.currentTarget.value))}>
          {#each printers as p}<option value={p.ID}>{p.Name} ({amortMethodLabel(p.AmortMethod)})</option>{/each}
        </select>
      </label>
    </div>
    {#if selSpool}<p class="note">{selSpool.mat.code} · {money(selSpool.spool.price, selSpool.spool.currency, $locale)}/кг{#if (selSpool.spool.leftG ?? selSpool.spool.weightLeftG) > 0}, {Math.round(selSpool.spool.leftG ?? selSpool.spool.weightLeftG)} {$locale === 'ru' ? 'г' : 'g'}{/if}</p>{/if}
    <div class="row">
      <label>{$t('calc.time_h')}<input placeholder="3" type="number" min="0" bind:value={timeH} oninput={recalc} /></label>
      <label>{$t('calc.time_m')}<input placeholder="30" type="number" min="0" max="59" bind:value={timeM} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.weight')}<input placeholder="135" type="number" min="0" step="0.01" bind:value={weightG} oninput={() => { filamentMM = null; recalc(); }} /></label>
      <label>{$t('calc.filament')}<input placeholder="42000" type="number" min="0" step="0.01" bind:value={filamentMM} oninput={() => { syncWeightFromFilament().then(recalc); }} /></label>
      <label>{$t('calc.purge')}<input placeholder="10" type="number" min="0" step="0.1" bind:value={purgeG} oninput={recalc} /></label>
    </div>
    <p class="note">{$t('calc.weight_filament_note')}</p>
    <h3 class="sec">{$t('calc.grp_machine')}</h3>
    <div class="row">
      <label>{$t('calc.tariff')}<input placeholder="7.08" type="number" min="0" step="0.01" bind:value={tariff} oninput={recalc} /></label>
      <label>{$t('calc.power')}<input placeholder="0.35" type="number" min="0" step="0.01" bind:value={powerKW} oninput={recalc} /></label>
      <label>{$t('calc.amort')}<input placeholder="5.41" type="number" min="0" step="0.01" bind:value={amortPerHour} oninput={recalc} /></label>
      <label>{$t('calc.maint')}<input placeholder="0" type="number" min="0" step="0.1" bind:value={maintPerHour} oninput={recalc} /></label>
    </div>
    <h3 class="sec">{$t('calc.grp_labor')}</h3>
    <div class="row">
      <label>{$t('calc.op_rate')}<input placeholder="100" type="number" min="0" bind:value={opRate} oninput={recalc} /></label>
      <label>{$t('calc.prep_h')}<input placeholder="0" type="number" min="0" bind:value={prep.h} oninput={() => { prep = normHM(prep.h, prep.m); recalc(); }} /></label>
      <label>{$t('calc.prep_m')}<input placeholder="0" type="number" min="0" max="59" bind:value={prep.m} oninput={() => { prep = normHM(prep.h, prep.m); recalc(); }} /></label>
      <label>{$t('calc.post_h')}<input placeholder="0" type="number" min="0" bind:value={post.h} oninput={() => { post = normHM(post.h, post.m); recalc(); }} /></label>
      <label>{$t('calc.post_m')}<input placeholder="0" type="number" min="0" max="59" bind:value={post.m} oninput={() => { post = normHM(post.h, post.m); recalc(); }} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.designer_rate')}<input placeholder="600" type="number" min="0" bind:value={designerRate} oninput={recalc} /></label>
      <label>{$t('calc.model_h')}<input placeholder="0" type="number" min="0" bind:value={model.h} oninput={() => { model = normHM(model.h, model.m); recalc(); }} /></label>
      <label>{$t('calc.model_m')}<input placeholder="0" type="number" min="0" max="59" bind:value={model.m} oninput={() => { model = normHM(model.h, model.m); recalc(); }} /></label>
      <label>{$t('calc.model_price')}<input placeholder="0" type="number" min="0" bind:value={modelPrice} oninput={recalc} /></label>
      <label>{$t('calc.extra')}<input placeholder="50" type="number" min="0" bind:value={extraMat} oninput={recalc} /></label>
    </div>
    <h3 class="sec">{$t('calc.grp_margins')}</h3>
    <div class="row">
      <label>{$t('calc.fail')}<input placeholder="5" type="number" min="0" max="100" bind:value={failPct} oninput={recalc} /></label>
      <label>{$t('calc.margin_profit')}<input placeholder="80" type="number" min="0" bind:value={marginProfitPct} oninput={recalc} /></label>
      <label>{$t('calc.margin_dev')}<input placeholder="35" type="number" min="0" bind:value={marginDevPct} oninput={recalc} /></label>
      <label>{$t('calc.fees')}<input placeholder="100" type="number" min="0" bind:value={fees} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.vat')}<input placeholder="20" type="number" min="0" bind:value={vatPct} oninput={recalc} /></label>
      <label>{$t('calc.round')}
        <select bind:value={roundStep} onchange={recalc}>
          <option value={0}>{$t('calc.round_none')}</option>
          <option value={1}>1</option>
          <option value={5}>5</option>
          <option value={10}>10</option>
          <option value={50}>50</option>
        </select>
      </label>
    </div>
  </section>

  <section class="card">
      <h3>{$t('calc.breakdown')}</h3>
      {#if result}
      <table>
        <tbody>{#each rows as [key, v]}<tr><td>{$t(key)}</td><td class="n">{money(v, currency, $locale)}</td></tr>{/each}</tbody>
      </table>
      <div class="totals">
        <div><span>{$t('calc.cost')}</span><b>{money(result.Cost, currency, $locale)}</b></div>
        <div><span>{$t('calc.price')}</span><b>{money(result.Price, currency, $locale)}</b></div>
        <div class="final"><span>{$t('calc.final')}</span><b>{money(result.PriceVAT, currency, $locale)}</b></div>
      </div>
      <div class="row">
        <label>{$t('calc.filename')}<input placeholder="part_PETG-NIT-W4_2h1m.gcode" bind:value={filename} /></label>
        <button onclick={save}>{$t('calc.save_calc')}</button>
      </div>
      <div class="row no-print">
        <button onclick={saveAsDefaults}>{$t('calc.setdefault')}</button>
        <button onclick={resetToDefaults}>{$t('calc.resetdef')}</button>
      </div>
      {#if orders.length}
        <div class="row">
          <label>{$t('cust.name')}
            <select value={customerId} onchange={(e) => pickCustomer(Number(e.currentTarget.value))}>
              <option value={0}>—</option>
              {#each customers as c}<option value={c.ID}>{c.Name}</option>{/each}
            </select>
          </label>
          <label>{$t('ord.number')}
            <select bind:value={orderId}>
              {#each visibleOrders as o}<option value={o.ID}>{orderLabel(o)}</option>{/each}
            </select>
          </label>
          <label>{$t('ord.item')}<input bind:value={orderTitle} placeholder={$t('ord.item_ph')} /></label>
          <label>{$t('ord.qty')}<input placeholder="1" type="number" min="0.5" step="0.5" bind:value={orderQty} /></label>
          <button onclick={addToOrder}>{$t('ord.add')}</button>
        </div>
        {#if addedInfo}<p class="ok">{$t('ord.added')}: {addedInfo}</p>{/if}
      {/if}
    {:else}<p>…</p>{/if}
  </section>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1.2fr 1fr; gap: 12px; }
  @media (max-width: 900px) { .grid { grid-template-columns: 1fr; } }
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; }
  h3.sec { font-size: 14px; margin: 12px 0 4px; color: var(--muted); }
  .note { color: var(--muted); font-size: 13px; margin: 2px 0 8px; }
  .err { color: var(--err); }
  .row { display: flex; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 110px; }
  input, select { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  .totals div { display: flex; justify-content: space-between; padding: 4px 0; }
  .final { font-size: 18px; border-top: 2px solid var(--final-border); margin-top: 4px; padding-top: 8px; }
  .ok { color: green; }
  .err { color: var(--err); }
  .warn { color: var(--warn); }
  button { padding: 8px 14px; align-self: end; }
</style>
