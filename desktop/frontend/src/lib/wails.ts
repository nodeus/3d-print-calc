// Тонкие обёртки над Wails-биндингами (window.go.main.App.*).
// Сгенерированного wailsjs нет специально: контракт — desktop/app.go.

export interface CalcInput {
  TotalSec: number;
  WeightG: number;
  PricePerKg: number;
  Tariff: number;
  PowerKW: number;
  AmortPerHour: number;
  OpRate: number;
  PrepH: number;
  PostH: number;
  DesignerRate: number;
  ModelH: number;
  ModelPrice: number;
  ExtraMat: number;
  FailRate: number;
  MarginProfit: number;
  MarginDev: number;
  Fees: number;
  MaintPerHour: number;
  PurgeWasteG: number;
  Copies: number;
  VATRate: number;
  RoundStep: number;
}

export interface CalcResult {
  MatCost: number;
  PurgeCost: number;
  ElecCost: number;
  Amort: number;
  Maint: number;
  Labor: number;
  Subtotal: number;
  FailAdj: number;
  Cost: number;
  Price: number;
  PriceRounded: number;
  PriceVAT: number;
  CostPerCopy: number;
  PricePerCopy: number;
}

export interface MaterialDTO {
  id: number;
  code: string;
  type: string;
  vendor: string;
  colorHex: string;
  note: string;
  density: number;
  diameter: number;
  defaultWeightG: number;
  pricePerKg: number;
  currency: string;
  spoolId: number;
  spoolLeftG: number;
}

export interface Spool {
  ID: number;
  MaterialID: number;
  Price: number;
  Currency: string;
  FxRate: number;
  BoughtAt: string;
  WeightInitG: number;
  WeightLeftG: number;
  Archived: boolean;
}

export interface Material {
  ID: number;
  Code: string;
  Type: string;
  Vendor: string;
  ColorHex: string;
  Density: number;
  Diameter: number;
  DefaultSpoolWeightG: number;
  Note: string;
}

export interface Printer {
  ID: number;
  Name: string;
  Model: string;
  PurchasePrice: number;
  Currency: string;
  CommissioningDate: string;
  LifeYears: number;
  HoursPerDay: number;
  CalendarMode: string;
  AmortMethod: string;
  PowerKW: number;
  MaintPerHour: number;
  ResourceHours: number;
  MoonrakerURL: string;
}

export interface PrinterDTO {
  ID: number;
  Name: string;
  Model: string;
  PurchasePrice: number;
  Currency: string;
  CommissioningDate: string;
  LifeYears: number;
  HoursPerDay: number;
  CalendarMode: string;
  AmortMethod: string;
  PowerKW: number;
  MaintPerHour: number;
  ResourceHours: number;
  MoonrakerURL: string;
  amortPerHour: number;
}

export interface JobDTO {
  ID: number;
  Filename: string;
  StartAt: string;
  EndAt: string;
  PrinterID: number;
  SpoolID: number;
  Status: string;
  Origin: string;
  Edited: number;
  PrintSec: number;
  TotalSec: number;
  EstimatedSec: number;
  SizeBytes: number;
  FilamentTotalMm: number;
  WeightG: number;
  LengthM: number;
  Slicer: string;
  CostSnapshot: string;
}

export interface JobInput {
  filename: string;
  printerId: number;
  spoolId: number;
  status: string;
  totalSec: number;
  weightG: number;
  lengthM: number;
  cost: number;
  price: number;
  currency: string;
  material: string;
  tariff: number;
  powerKW: number;
  amortPerHour: number;
  opRate: number;
  prepH: number;
  postH: number;
}

export interface JobEdit {
  id: number;
  filename: string;
  status: string;
  startAt: string;
  endAt: string;
  printSec: number;
  totalSec: number;
  estimatedSec: number;
  sizeBytes: number;
  weightG: number;
  lengthM: number;
  slicer: string;
  material: string;
}

export interface BackupManifest {
  version: number;
  created_at: string;
  counts: Record<string, number>;
  files?: string[];
}

export interface ImportReport {
  total: number;
  new: number;
  existing: number;
  stubs: number;
  dryRun: boolean;
}

export interface UserDTO {
  id: number;
  login: string;
  role: string;
  locale: string;
}

export interface Customer {
  ID: number;
  Name: string;
  Contact: string;
  Currency: string;
  Note: string;
}

export interface Order {
  ID: number;
  CustomerID: number;
  Number: string;
  Title: string;
  Currency: string;
  FxRate: number;
  MarginProfit: number;
  MarginDev: number;
  VAT: number;
  DiscountPct: number;
  DiscountFixed: number;
  Status: string;
  TotalBase: number;
  CreatedAt: string;
}

export interface OrderItem {
  ID: number;
  OrderID: number;
  JobID: number;
  Title: string;
  Qty: number;
  Cost: number;
  Price: number;
}

export interface Payment {
  ID: number;
  OrderID: number;
  OrderItem: number;
  Amount: number;
  PaidAt: string;
  Note: string;
}

export interface OrderDetail {
  order: Order;
  items: OrderItem[];
  payments: Payment[];
  paid: number;
  balance: number;
}

export interface OrderRow {
  order: Order;
  paid: number;
  balance: number;
  payState: string;
}

export interface MonthRevenue {
  month: string;
  total: number;
  orders: number;
}

export interface MaterialStat {
  code: string;
  jobs: number;
  weight: number;
  cost: number;
}

export interface PrinterStat {
  name: string;
  jobs: number;
  totalSec: number;
  lengthM: number;
  weightG: number;
  remainH: number;
}

export interface Debt {
  orderId: number;
  orderNo: string;
  customer: string;
  title: string;
  total: number;
  paid: number;
  balance: number;
  status: string;
}

export interface SpoolStat {
  code: string;
  colorHex: string;
  vendor: string;
  jobs: number;
  usedG: number;
  initG: number;
  leftG: number;
}

export interface PDFSection {
  Title: string;
  Headers: string[];
  Rows: string[][];
}

declare global {
  interface Window {
    runtime: {
      EventsOn(event: string, callback: (data: never) => void): void;
      EventsOff(event: string): void;
    };
    go: {
      main: {
        App: {
          Calc(input: CalcInput): Promise<CalcResult>;
          WeightFromLength(density: number, diameterMM: number, lengthMM: number): Promise<number>;
          GetMaterials(): Promise<MaterialDTO[]>;
          GetPrinters(): Promise<PrinterDTO[]>;
          GetJobs(limit: number): Promise<JobDTO[]>;
          SaveJob(input: JobInput): Promise<number>;
          GetLocale(): Promise<string>;
          SetLocale(locale: string): Promise<void>;
          GetBaseCurrency(): Promise<string>;
          SetBaseCurrency(code: string): Promise<void>;
          ExportBackup(): Promise<string>;
          PreviewBackup(): Promise<BackupManifest>;
          ApplyBackup(): Promise<BackupManifest>;
          ExportJobsCSV(): Promise<string>;
          ExportMaterialsCSV(): Promise<string>;
          PreviewHistoryCSV(): Promise<ImportReport>;
          ApplyHistoryCSV(): Promise<ImportReport>;
          Login(login: string, password: string): Promise<UserDTO>;
          Logout(): Promise<void>;
          CurrentUser(): Promise<UserDTO | null>;
          ListUsers(): Promise<UserDTO[]>;
          CreateUser(login: string, password: string, role: string): Promise<number>;
          SetUserRole(id: number, role: string): Promise<void>;
          ChangeMyPassword(oldPw: string, pw: string): Promise<void>;
          ListCustomers(): Promise<Customer[]>;
          SaveCustomer(c: Customer): Promise<number>;
          CreateOrder(customerId: number, currency: string, title: string): Promise<number>;
          ListOrders(): Promise<Order[]>;
          ListOrderRows(): Promise<OrderRow[]>;
          GetOrderDetail(id: number): Promise<OrderDetail>;
          SetOrderStatus(id: number, status: string): Promise<void>;
          UpdateOrder(o: Order): Promise<void>;
          DeleteOrder(id: number): Promise<void>;
          DeleteJob(id: number): Promise<void>;
          UpdateManualJob(job: JobEdit): Promise<void>;
          OrderNumberSettings(): Promise<Record<string, string>>;
          SetOrderNumberSettings(template: string, seq: string): Promise<void>;
          PreviewOrderNumber(template: string): Promise<string>;
          OpenURL(url: string): Promise<void>;
          AddOrderItem(item: { orderId: number; jobId: number; title: string; qty: number; cost: number; price: number }): Promise<number>;
          DeleteOrderItem(id: number): Promise<void>;
          DeleteOrderItem(id: number): Promise<void>;
          AddPayment(orderId: number, itemId: number, amount: number, note: string): Promise<number>;
          RevenueByMonth(): Promise<MonthRevenue[]>;
          TopMaterials(): Promise<MaterialStat[]>;
          PrinterLoad(): Promise<PrinterStat[]>;
          SpoolStats(): Promise<SpoolStat[]>;
          Debts(): Promise<Debt[]>;
          SetMoonrakerURL(printerId: number, url: string): Promise<void>;
          MoonrakerStatus(printerId: number): Promise<string>;
          MoonrakerPreview(printerId: number, limit: number): Promise<ImportReport>;
          MoonrakerApply(): Promise<ImportReport>;
          AddMaterial(code: string, matType: string, density: number, pricePerKg: number, weightG: number): Promise<number>;
          UpdateMaterial(m: Material): Promise<void>;
          DeleteMaterial(id: number): Promise<void>;
          UpdateSpool(s: Spool): Promise<void>;
          SetMaterialPrice(materialId: number, priceMinor: number, currency: string): Promise<void>;
          GetMaterialColorFavs(): Promise<string[]>;
          SetMaterialColorFavs(cols: string[]): Promise<void>;
          AddPrinter(p: Printer): Promise<number>;
          UpdatePrinter(p: Printer): Promise<void>;
          DeletePrinter(id: number): Promise<void>;
          DeleteCustomer(id: number): Promise<void>;
          DeleteUser(id: number): Promise<void>;
          AppVersion(): Promise<Record<string, string>>;
          UISettings(): Promise<{ theme: string; custom: Record<string, Record<string, string>> }>;
          SetUITheme(name: string): Promise<void>;
          SaveCustomTheme(name: string, vars: Record<string, string>): Promise<void>;
          DeleteCustomTheme(name: string): Promise<void>;
          ExportThemeFile(name: string): Promise<string>;
          ImportThemeFile(): Promise<string>;
          GetHistoryColumns(): Promise<string[]>;
          SetHistoryColumns(cols: string[]): Promise<void>;
          ExportPDF(title: string, filename: string, sections: PDFSection[]): Promise<string>;
        };
      };
    }
  }
}

const api = () => window.go.main.App;

export const wails = {
  calc: (input: CalcInput) => api().Calc(input),
  weightFromLength: (density: number, diameterMM: number, lengthMM: number) =>
    api().WeightFromLength(density, diameterMM, lengthMM),
  materials: () => api().GetMaterials(),
  printers: () => api().GetPrinters(),
  jobs: (limit: number) => api().GetJobs(limit),
  saveJob: (input: JobInput) => api().SaveJob(input),
  getLocale: () => api().GetLocale(),
  setLocale: (locale: string) => api().SetLocale(locale),
  getBaseCurrency: () => api().GetBaseCurrency(),
  setBaseCurrency: (code: string) => api().SetBaseCurrency(code),
  exportBackup: () => api().ExportBackup(),
  previewBackup: () => api().PreviewBackup(),
  applyBackup: () => api().ApplyBackup(),
  exportJobsCSV: () => api().ExportJobsCSV(),
  exportMaterialsCSV: () => api().ExportMaterialsCSV(),
  previewHistoryCSV: () => api().PreviewHistoryCSV(),
  applyHistoryCSV: () => api().ApplyHistoryCSV(),
  login: (login: string, password: string) => api().Login(login, password),
  logout: () => api().Logout(),
  currentUser: () => api().CurrentUser(),
  listUsers: () => api().ListUsers(),
  createUser: (login: string, password: string, role: string) => api().CreateUser(login, password, role),
  setUserRole: (id: number, role: string) => api().SetUserRole(id, role),
  changeMyPassword: (oldPw: string, pw: string) => api().ChangeMyPassword(oldPw, pw),
  listCustomers: () => api().ListCustomers(),
  saveCustomer: (c: Customer) => api().SaveCustomer(c),
  createOrder: (customerId: number, currency: string, title: string) =>
    api().CreateOrder(customerId, currency, title),
  listOrders: () => api().ListOrders(),
  listOrderRows: () => api().ListOrderRows(),
  orderDetail: (id: number) => api().GetOrderDetail(id),
  setOrderStatus: (id: number, status: string) => api().SetOrderStatus(id, status),
  updateOrder: (o: Order) => api().UpdateOrder(o),
  deleteOrder: (id: number) => api().DeleteOrder(id),
  deleteJob: (id: number) => api().DeleteJob(id),
  updateManualJob: (job: JobEdit) => api().UpdateManualJob(job),
  orderNumberSettings: () => api().OrderNumberSettings(),
  setOrderNumberSettings: (template: string, seq: string) => api().SetOrderNumberSettings(template, seq),
  previewOrderNumber: (template: string) => api().PreviewOrderNumber(template),
  openURL: (url: string) => api().OpenURL(url),
  addOrderItem: (item: { orderId: number; jobId: number; title: string; qty: number; cost: number; price: number }) =>
    api().AddOrderItem(item),
  deleteOrderItem: (id: number) => api().DeleteOrderItem(id),
  addPayment: (orderId: number, itemId: number, amount: number, note: string) =>
    api().AddPayment(orderId, itemId, amount, note),
  deleteOrderItem: (id: number) => api().DeleteOrderItem(id),
  revenueByMonth: () => api().RevenueByMonth(),
  topMaterials: () => api().TopMaterials(),
  printerLoad: () => api().PrinterLoad(),
  spoolStats: () => api().SpoolStats(),
  debts: () => api().Debts(),
  setMoonrakerURL: (printerId: number, url: string) => api().SetMoonrakerURL(printerId, url),
  moonrakerStatus: (printerId: number) => api().MoonrakerStatus(printerId),
  moonrakerPreview: (printerId: number, limit: number) => api().MoonrakerPreview(printerId, limit),
  moonrakerApply: () => api().MoonrakerApply(),
  addMaterial: (code: string, matType: string, density: number, pricePerKg: number, weightG: number) =>
    api().AddMaterial(code, matType, density, pricePerKg, weightG),
  updateMaterial: (m: Material) => api().UpdateMaterial(m),
  deleteMaterial: (id: number) => api().DeleteMaterial(id),
  updateSpool: (s: Spool) => api().UpdateSpool(s),
  setMaterialPrice: (materialId: number, priceMinor: number, currency: string) =>
    api().SetMaterialPrice(materialId, priceMinor, currency),
  getMaterialColorFavs: () => api().GetMaterialColorFavs(),
  setMaterialColorFavs: (cols: string[]) => api().SetMaterialColorFavs(cols),
  addPrinter: (p: Printer) => api().AddPrinter(p),
  updatePrinter: (p: Printer) => api().UpdatePrinter(p),
  deletePrinter: (id: number) => api().DeletePrinter(id),
  deleteCustomer: (id: number) => api().DeleteCustomer(id),
  deleteUser: (id: number) => api().DeleteUser(id),
  appVersion: () => api().AppVersion(),
  uiSettings: () => api().UISettings(),
  setUITheme: (name: string) => api().SetUITheme(name),
  saveCustomTheme: (name: string, vars: Record<string, string>) => api().SaveCustomTheme(name, vars),
  deleteCustomTheme: (name: string) => api().DeleteCustomTheme(name),
  exportThemeFile: (name: string) => api().ExportThemeFile(name),
  importThemeFile: () => api().ImportThemeFile(),
  historyColumns: () => api().GetHistoryColumns(),
  setHistoryColumns: (cols: string[]) => api().SetHistoryColumns(cols),
  exportPDF: (title: string, filename: string, sections: PDFSection[]) =>
    api().ExportPDF(title, filename, sections),
  onImportProgress: (cb: (done: number, total: number) => void): void => {
    try {
      window.runtime.EventsOn('import-progress', (d: unknown) => {
        const m = d as { done: number; total: number };
        cb(m.done, m.total);
      });
    } catch {
      /* не под wails (dev) — прогресса нет */
    }
  },
  offImportProgress: (): void => {
    try {
      window.runtime.EventsOff('import-progress');
    } catch {
      /* noop */
    }
  },
};
