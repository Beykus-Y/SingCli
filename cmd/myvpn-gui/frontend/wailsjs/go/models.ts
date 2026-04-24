export namespace main {
	
	export class ServerSummary {
	    name: string;
	    type: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.address = source["address"];
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

}

