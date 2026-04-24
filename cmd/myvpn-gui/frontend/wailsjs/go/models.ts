export namespace config {
	
	export class RealityConfig {
	    publicKey: string;
	    shortId: string;
	    fingerprint: string;
	    serverName: string;
	    spiderX: string;
	
	    static createFrom(source: any = {}) {
	        return new RealityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.publicKey = source["publicKey"];
	        this.shortId = source["shortId"];
	        this.fingerprint = source["fingerprint"];
	        this.serverName = source["serverName"];
	        this.spiderX = source["spiderX"];
	    }
	}
	export class TLSConfig {
	    enabled: boolean;
	    insecure: boolean;
	    serverName: string;
	
	    static createFrom(source: any = {}) {
	        return new TLSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.insecure = source["insecure"];
	        this.serverName = source["serverName"];
	    }
	}
	export class ServerEntry {
	    name: string;
	    type: string;
	    server: string;
	    uuid: string;
	    password: string;
	    username: string;
	    tls: TLSConfig;
	    reality: RealityConfig;
	    insecure: boolean;
	    allowInsecure: boolean;
	    upMbps: number;
	    downMbps: number;
	    network: string;
	    path: string;
	    host: string;
	    alterId: number;
	    flow: string;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.server = source["server"];
	        this.uuid = source["uuid"];
	        this.password = source["password"];
	        this.username = source["username"];
	        this.tls = this.convertValues(source["tls"], TLSConfig);
	        this.reality = this.convertValues(source["reality"], RealityConfig);
	        this.insecure = source["insecure"];
	        this.allowInsecure = source["allowInsecure"];
	        this.upMbps = source["upMbps"];
	        this.downMbps = source["downMbps"];
	        this.network = source["network"];
	        this.path = source["path"];
	        this.host = source["host"];
	        this.alterId = source["alterId"];
	        this.flow = source["flow"];
	        this.method = source["method"];
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

export namespace main {
	
	export class ServerSummary {
	    id: number;
	    name: string;
	    type: string;
	    address: string;
	    subscriptionId?: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.address = source["address"];
	        this.subscriptionId = source["subscriptionId"];
	    }
	}
	export class AppState {
	    servers: ServerSummary[];
	    selectedServer: string;
	    mode: string;
	    status: string;
	    running: boolean;
	    connecting: boolean;
	    canConnect: boolean;
	    canDisconnect: boolean;
	    logs: string[];
	    proxySocks: string;
	    proxyHTTP: string;
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.servers = this.convertValues(source["servers"], ServerSummary);
	        this.selectedServer = source["selectedServer"];
	        this.mode = source["mode"];
	        this.status = source["status"];
	        this.running = source["running"];
	        this.connecting = source["connecting"];
	        this.canConnect = source["canConnect"];
	        this.canDisconnect = source["canDisconnect"];
	        this.logs = source["logs"];
	        this.proxySocks = source["proxySocks"];
	        this.proxyHTTP = source["proxyHTTP"];
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
	export class PingResult {
	    latencyMs: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}

}

export namespace storage {
	
	export class ImportResult {
	    imported: boolean;
	    path: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.path = source["path"];
	        this.count = source["count"];
	    }
	}
	export class ServerInput {
	    server: config.ServerEntry;
	    subscriptionId?: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], config.ServerEntry);
	        this.subscriptionId = source["subscriptionId"];
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
	export class ServerRecord {
	    id: number;
	    name: string;
	    type: string;
	    address: string;
	    server: config.ServerEntry;
	    subscriptionId?: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.address = source["address"];
	        this.server = this.convertValues(source["server"], config.ServerEntry);
	        this.subscriptionId = source["subscriptionId"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
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
	export class Subscription {
	    id: number;
	    name: string;
	    url: string;
	    enabled: boolean;
	    autoUpdateIntervalMinutes: number;
	    profileUpdateIntervalMinutes?: number;
	    lastCheckedAt?: string;
	    lastUpdatedAt?: string;
	    lastError?: string;
	    uploadBytes?: number;
	    downloadBytes?: number;
	    usedBytes?: number;
	    totalBytes?: number;
	    expireAt?: string;
	    profileTitle?: string;
	    profileWebPageUrl?: string;
	    supportUrl?: string;
	    headersJson?: string;
	    etag?: string;
	    lastModified?: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Subscription(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.enabled = source["enabled"];
	        this.autoUpdateIntervalMinutes = source["autoUpdateIntervalMinutes"];
	        this.profileUpdateIntervalMinutes = source["profileUpdateIntervalMinutes"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.lastUpdatedAt = source["lastUpdatedAt"];
	        this.lastError = source["lastError"];
	        this.uploadBytes = source["uploadBytes"];
	        this.downloadBytes = source["downloadBytes"];
	        this.usedBytes = source["usedBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.expireAt = source["expireAt"];
	        this.profileTitle = source["profileTitle"];
	        this.profileWebPageUrl = source["profileWebPageUrl"];
	        this.supportUrl = source["supportUrl"];
	        this.headersJson = source["headersJson"];
	        this.etag = source["etag"];
	        this.lastModified = source["lastModified"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class SubscriptionInput {
	    name: string;
	    url: string;
	    enabled: boolean;
	    autoUpdateIntervalMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.enabled = source["enabled"];
	        this.autoUpdateIntervalMinutes = source["autoUpdateIntervalMinutes"];
	    }
	}
	export class SubscriptionRefreshResult {
	    subscription: Subscription;
	    serverCount: number;
	    updated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionRefreshResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subscription = this.convertValues(source["subscription"], Subscription);
	        this.serverCount = source["serverCount"];
	        this.updated = source["updated"];
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

