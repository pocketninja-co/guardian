export namespace main {
	
	export class ScanResult {
	    riskScore: number;
	    findings: string[];
	    isClean: boolean;
	    certificate?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.riskScore = source["riskScore"];
	        this.findings = source["findings"];
	        this.isClean = source["isClean"];
	        this.certificate = source["certificate"];
	    }
	}

}

export namespace risk {
	
	export class RiskProfile {
	    filePath: string;
	    riskScore: number;
	    ssnCount: number;
	    hasDiagnosis: boolean;
	    riskLabel: string;
	    estimatedFine: number;
	    findings: string[];
	
	    static createFrom(source: any = {}) {
	        return new RiskProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.riskScore = source["riskScore"];
	        this.ssnCount = source["ssnCount"];
	        this.hasDiagnosis = source["hasDiagnosis"];
	        this.riskLabel = source["riskLabel"];
	        this.estimatedFine = source["estimatedFine"];
	        this.findings = source["findings"];
	    }
	}
	export class AuditReport {
	    totalFiles: number;
	    totalRiskScore: number;
	    potentialLiability: number;
	    topOffenders: RiskProfile[];
	    criticalCount: number;
	
	    static createFrom(source: any = {}) {
	        return new AuditReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFiles = source["totalFiles"];
	        this.totalRiskScore = source["totalRiskScore"];
	        this.potentialLiability = source["potentialLiability"];
	        this.topOffenders = this.convertValues(source["topOffenders"], RiskProfile);
	        this.criticalCount = source["criticalCount"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace storage {
	
	export class AuditEntry {
	    timestamp: string;
	    total_files: number;
	    risk_score: number;
	    user: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.total_files = source["total_files"];
	        this.risk_score = source["risk_score"];
	        this.user = source["user"];
	        this.status = source["status"];
	    }
	}
	export class ScheduleConfig {
	    schedule_enabled: boolean;
	    scan_interval_hours: number;
	    interval_value: number;
	    interval_unit: string;
	    time_of_day: string;
	    timezone: string;
	    scan_paths: string[];
	    audit_history: AuditEntry[];
	    // Go type: time
	    last_notification?: any;
	    total_files_scanned: number;
	    total_risks_found: number;
	    total_liability: number;
	
	    static createFrom(source: any = {}) {
	        return new ScheduleConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schedule_enabled = source["schedule_enabled"];
	        this.scan_interval_hours = source["scan_interval_hours"];
	        this.interval_value = source["interval_value"];
	        this.interval_unit = source["interval_unit"];
	        this.time_of_day = source["time_of_day"];
	        this.timezone = source["timezone"];
	        this.scan_paths = source["scan_paths"];
	        this.audit_history = this.convertValues(source["audit_history"], AuditEntry);
	        this.last_notification = this.convertValues(source["last_notification"], null);
	        this.total_files_scanned = source["total_files_scanned"];
	        this.total_risks_found = source["total_risks_found"];
	        this.total_liability = source["total_liability"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

