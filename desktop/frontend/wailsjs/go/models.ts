export namespace calc {
	
	export class Input {
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
	
	    static createFrom(source: any = {}) {
	        return new Input(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TotalSec = source["TotalSec"];
	        this.WeightG = source["WeightG"];
	        this.PricePerKg = source["PricePerKg"];
	        this.Tariff = source["Tariff"];
	        this.PowerKW = source["PowerKW"];
	        this.AmortPerHour = source["AmortPerHour"];
	        this.OpRate = source["OpRate"];
	        this.PrepH = source["PrepH"];
	        this.PostH = source["PostH"];
	        this.DesignerRate = source["DesignerRate"];
	        this.ModelH = source["ModelH"];
	        this.ModelPrice = source["ModelPrice"];
	        this.ExtraMat = source["ExtraMat"];
	        this.FailRate = source["FailRate"];
	        this.MarginProfit = source["MarginProfit"];
	        this.MarginDev = source["MarginDev"];
	        this.Fees = source["Fees"];
	        this.MaintPerHour = source["MaintPerHour"];
	        this.PurgeWasteG = source["PurgeWasteG"];
	        this.Copies = source["Copies"];
	        this.VATRate = source["VATRate"];
	        this.RoundStep = source["RoundStep"];
	    }
	}
	export class Result {
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
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MatCost = source["MatCost"];
	        this.PurgeCost = source["PurgeCost"];
	        this.ElecCost = source["ElecCost"];
	        this.Amort = source["Amort"];
	        this.Maint = source["Maint"];
	        this.Labor = source["Labor"];
	        this.Subtotal = source["Subtotal"];
	        this.FailAdj = source["FailAdj"];
	        this.Cost = source["Cost"];
	        this.Price = source["Price"];
	        this.PriceRounded = source["PriceRounded"];
	        this.PriceVAT = source["PriceVAT"];
	        this.CostPerCopy = source["CostPerCopy"];
	        this.PricePerCopy = source["PricePerCopy"];
	    }
	}

}

export namespace main {
	
	export class JobInput {
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
	
	    static createFrom(source: any = {}) {
	        return new JobInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.printerId = source["printerId"];
	        this.spoolId = source["spoolId"];
	        this.status = source["status"];
	        this.totalSec = source["totalSec"];
	        this.weightG = source["weightG"];
	        this.lengthM = source["lengthM"];
	        this.cost = source["cost"];
	        this.price = source["price"];
	        this.currency = source["currency"];
	    }
	}
	export class MaterialDTO {
	    id: number;
	    code: string;
	    type: string;
	    density: number;
	    diameter: number;
	    pricePerKg: number;
	    currency: string;
	
	    static createFrom(source: any = {}) {
	        return new MaterialDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.type = source["type"];
	        this.density = source["density"];
	        this.diameter = source["diameter"];
	        this.pricePerKg = source["pricePerKg"];
	        this.currency = source["currency"];
	    }
	}
	export class PrinterDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new PrinterDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Model = source["Model"];
	        this.PurchasePrice = source["PurchasePrice"];
	        this.Currency = source["Currency"];
	        this.CommissioningDate = source["CommissioningDate"];
	        this.LifeYears = source["LifeYears"];
	        this.HoursPerDay = source["HoursPerDay"];
	        this.CalendarMode = source["CalendarMode"];
	        this.AmortMethod = source["AmortMethod"];
	        this.PowerKW = source["PowerKW"];
	        this.MaintPerHour = source["MaintPerHour"];
	        this.ResourceHours = source["ResourceHours"];
	        this.amortPerHour = source["amortPerHour"];
	    }
	}

}

export namespace models {
	
	export class Job {
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
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Filename = source["Filename"];
	        this.StartAt = source["StartAt"];
	        this.PrinterID = source["PrinterID"];
	        this.SpoolID = source["SpoolID"];
	        this.Status = source["Status"];
	        this.PrintSec = source["PrintSec"];
	        this.TotalSec = source["TotalSec"];
	        this.WeightG = source["WeightG"];
	        this.LengthM = source["LengthM"];
	        this.CostSnapshot = source["CostSnapshot"];
	    }
	}

}

