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
  density: number;
  diameter: number;
  pricePerKg: number;
  currency: string;
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
  amortPerHour: number;
}

export interface JobDTO {
  ID: number;
  Filename: string;
  StartAt: string;
  PrinterID: number;
  SpoolID: number;
  Status: string;
  PrintSec: number;
  TotalSec: number;
  WeightG: number;
  LengthM: number;
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
};
