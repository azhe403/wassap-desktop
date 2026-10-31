export namespace browser {
	
	export class Evidence {
	    Key: string;
	    Label: string;
	    Value: string;
	    Relevance: string;
	
	    static createFrom(source: any = {}) {
	        return new Evidence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Label = source["Label"];
	        this.Value = source["Value"];
	        this.Relevance = source["Relevance"];
	    }
	}
	export class StorageInfo {
	    persisted: boolean;
	    bytes: number;
	    cookies: number;
	    partial: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.persisted = source["persisted"];
	        this.bytes = source["bytes"];
	        this.cookies = source["cookies"];
	        this.partial = source["partial"];
	    }
	}
	export class Info {
	    id: string;
	    path: string;
	    userData: string;
	    state: number;
	    open: boolean;
	    storage: StorageInfo;
	    createdAt: string;
	    lastError: string;
	    layout: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.userData = source["userData"];
	        this.state = source["state"];
	        this.open = source["open"];
	        this.storage = this.convertValues(source["storage"], StorageInfo);
	        this.createdAt = source["createdAt"];
	        this.lastError = source["lastError"];
	        this.layout = source["layout"];
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

