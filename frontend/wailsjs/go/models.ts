export namespace account {
	
	export class Account {
	    id: string;
	    name: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

export namespace config {
	
	export class PrivacyBlurConfig {
	    enabled: boolean;
	    privacyMode: string;
	    blurAmount: number;
	    animation: boolean;
	    animationDuration: number;
	    inputOpacity: number;
	    unblurLastN: boolean;
	    unblurLastNCount: number;
	    blurAvatars: boolean;
	    blurNames: boolean;
	    blurPreviews: boolean;
	    blurTextChats: boolean;
	    blurStickers: boolean;
	    blurMediaPreview: boolean;
	    blurMediaGallery: boolean;
	    blurInput: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrivacyBlurConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.privacyMode = source["privacyMode"];
	        this.blurAmount = source["blurAmount"];
	        this.animation = source["animation"];
	        this.animationDuration = source["animationDuration"];
	        this.inputOpacity = source["inputOpacity"];
	        this.unblurLastN = source["unblurLastN"];
	        this.unblurLastNCount = source["unblurLastNCount"];
	        this.blurAvatars = source["blurAvatars"];
	        this.blurNames = source["blurNames"];
	        this.blurPreviews = source["blurPreviews"];
	        this.blurTextChats = source["blurTextChats"];
	        this.blurStickers = source["blurStickers"];
	        this.blurMediaPreview = source["blurMediaPreview"];
	        this.blurMediaGallery = source["blurMediaGallery"];
	        this.blurInput = source["blurInput"];
	    }
	}

}

export namespace main {
	
	export class ProfileResourceUsage {
	    id: string;
	    running: boolean;
	    processCount: number;
	    workingSetBytes: number;
	    cpuPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new ProfileResourceUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.running = source["running"];
	        this.processCount = source["processCount"];
	        this.workingSetBytes = source["workingSetBytes"];
	        this.cpuPercent = source["cpuPercent"];
	    }
	}
	export class ResourceUsage {
	    appWorkingSetBytes: number;
	    appCPUPercent: number;
	    totalWorkingSetBytes: number;
	    totalCPUPercent: number;
	    profiles: ProfileResourceUsage[];
	
	    static createFrom(source: any = {}) {
	        return new ResourceUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appWorkingSetBytes = source["appWorkingSetBytes"];
	        this.appCPUPercent = source["appCPUPercent"];
	        this.totalWorkingSetBytes = source["totalWorkingSetBytes"];
	        this.totalCPUPercent = source["totalCPUPercent"];
	        this.profiles = this.convertValues(source["profiles"], ProfileResourceUsage);
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
	export class UpdateStatus {
	    state: string;
	    channel: string;
	    currentVersion: string;
	    latestVersion: string;
	    releaseURL: string;
	    message?: string;
	    checkedAt?: string;
	    progressDone?: number;
	    progressTotal?: number;
	    dismissed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.channel = source["channel"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.releaseURL = source["releaseURL"];
	        this.message = source["message"];
	        this.checkedAt = source["checkedAt"];
	        this.progressDone = source["progressDone"];
	        this.progressTotal = source["progressTotal"];
	        this.dismissed = source["dismissed"];
	    }
	}
	export class VersionInfo {
	    version: string;
	    commit: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.commit = source["commit"];
	    }
	}

}

