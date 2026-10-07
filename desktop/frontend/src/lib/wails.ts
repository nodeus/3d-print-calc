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
  Currency: string;
  FxRate: number;
  MarginProfit: number;
  MarginDev: number;
  VAT: number;
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
}

export interface Debt {
  orderId: number;
  customer: string;
  total: number;
  paid: number;
  balance: number;
  status: string;
}

declare global {
  interface Window {
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
          CreateOrder(customerId: number, currency: string): Promise<number>;
          ListOrders(): Promise<Order[]>;
          GetOrderDetail(id: number): Promise<OrderDetail>;
          SetOrderStatus(id: number, status: string): Promise<void>;
          AddOrderItem(item: { orderId: number; jobId: number; title: string; qty: number; cost: number; price: number }): Promise<number>;
          AddPayment(orderId: number, amount: number, note: string): Promise<number>;
          RevenueByMonth(): Promise<MonthRevenue[]>;
          TopMaterials(): Promise<MaterialStat[]>;
          PrinterLoad(): Promise<PrinterStat[]>;
          Debts(): Promise<Debt[]>;
          SetMoonrakerURL(printerId: number, url: string): Promise<void>;
          MoonrakerStatus(printerId: number): Promise<string>;
          MoonrakerPreview(printerId: number, limit: number): Promise<ImportReport>;
          MoonrakerApply(): Promise<ImportReport>;
          AddMaterial(code: string, matType: string, density: number, pricePerKg: number): Promise<number>;
          UpdateMaterial(m: Material): Promise<void>;
          DeleteMaterial(id: number): Promise<void>;
          UpdateSpool(s: Spool): Promise<void>;
          SetMaterialPrice(materialId: number, priceMinor: number, currency: string): Promise<void>;
          AddPrinter(p: Printer): Promise<number>;
          UpdatePrinter(p: Printer): Promise<void>;
          DeletePrinter(id: number): Promise<void>;
          DeleteCustomer(id: number): Promise<void>;
          DeleteUser(id: number): Promise<void>;
          AppVersion(): Promise<Record<string, string>>;
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
  createOrder: (customerId: number, currency: string) => api().CreateOrder(customerId, currency),
  listOrders: () => api().ListOrders(),
  orderDetail: (id: number) => api().GetOrderDetail(id),
  setOrderStatus: (id: number, status: string) => api().SetOrderStatus(id, status),
  addOrderItem: (item: { orderId: number; jobId: number; title: string; qty: number; cost: number; price: number }) =>
    api().AddOrderItem(item),
  addPayment: (orderId: number, amount: number, note: string) => api().AddPayment(orderId, amount, note),
  revenueByMonth: () => api().RevenueByMonth(),
  topMaterials: () => api().TopMaterials(),
  printerLoad: () => api().PrinterLoad(),
  debts: () => api().Debts(),
  setMoonrakerURL: (printerId: number, url: string) => api().SetMoonrakerURL(printerId, url),
  moonrakerStatus: (printerId: number) => api().MoonrakerStatus(printerId),
  moonrakerPreview: (printerId: number, limit: number) => api().MoonrakerPreview(printerId, limit),
  moonrakerApply: () => api().MoonrakerApply(),
  addMaterial: (code: string, matType: string, density: number, pricePerKg: number) =>
    api().AddMaterial(code, matType, density, pricePerKg),
  updateMaterial: (m: Material) => api().UpdateMaterial(m),
  deleteMaterial: (id: number) => api().DeleteMaterial(id),
  updateSpool: (s: Spool) => api().UpdateSpool(s),
  setMaterialPrice: (materialId: number, priceMinor: number, currency: string) =>
    api().SetMaterialPrice(materialId, priceMinor, currency),
  addPrinter: (p: Printer) => api().AddPrinter(p),
  updatePrinter: (p: Printer) => api().UpdatePrinter(p),
  deletePrinter: (id: number) => api().DeletePrinter(id),
  deleteCustomer: (id: number) => api().DeleteCustomer(id),
  deleteUser: (id: number) => api().DeleteUser(id),
  appVersion: () => api().AppVersion(),
};
