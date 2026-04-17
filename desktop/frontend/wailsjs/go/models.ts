export namespace beamsync {
	
	export class DeviceRule {
	    ip: string;
	    friendlyName: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.friendlyName = source["friendlyName"];
	    }
	}
	export class TransferSettings {
	    mode: string;
	    maxFileSizeMB: number;
	    blockedExtensions: string[];
	    trustedDevices: DeviceRule[];
	    blockedDevices: DeviceRule[];
	
	    static createFrom(source: any = {}) {
	        return new TransferSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.maxFileSizeMB = source["maxFileSizeMB"];
	        this.blockedExtensions = source["blockedExtensions"];
	        this.trustedDevices = this.convertValues(source["trustedDevices"], DeviceRule);
	        this.blockedDevices = this.convertValues(source["blockedDevices"], DeviceRule);
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
	
	export class ReceivedFile {
	    name: string;
	    sizeBytes: number;
	    modTime: string;
	
	    static createFrom(source: any = {}) {
	        return new ReceivedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sizeBytes = source["sizeBytes"];
	        this.modTime = source["modTime"];
	    }
	}

}

