export namespace main {
	
	export class AppInfo {
	    os: string;
	    needAdmin: boolean;
	    postgresInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.needAdmin = source["needAdmin"];
	        this.postgresInstalled = source["postgresInstalled"];
	    }
	}
	export class InstallDirConflict {
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstallDirConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	    }
	}
	export class OkidociCheckResult {
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OkidociCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	    }
	}
	export class PgCheckResult {
	    installed: boolean;
	    installerFound: boolean;
	    installerPath: string;
	
	    static createFrom(source: any = {}) {
	        return new PgCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.installerFound = source["installerFound"];
	        this.installerPath = source["installerPath"];
	    }
	}
	export class ServerStatusDTO {
	    taskExists: boolean;
	    running: boolean;
	    raw?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskExists = source["taskExists"];
	        this.running = source["running"];
	        this.raw = source["raw"];
	    }
	}
	export class ServerInfoDTO {
	    status: ServerStatusDTO;
	    connections?: string;
	    version?: string;
	    sessionCount: number;
	    consoleErr?: string;
	    rawConsole?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], ServerStatusDTO);
	        this.connections = source["connections"];
	        this.version = source["version"];
	        this.sessionCount = source["sessionCount"];
	        this.consoleErr = source["consoleErr"];
	        this.rawConsole = source["rawConsole"];
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
	
	export class StartInstallOptions {
	    installServer: boolean;
	    installClients: boolean;
	    installDB: boolean;
	    sourceRoot: string;
	    postgresPassword: string;
	    installDir: string;
	    dbAction: string;
	    restoreSqlPath: string;
	    reinstallExisting: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StartInstallOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installServer = source["installServer"];
	        this.installClients = source["installClients"];
	        this.installDB = source["installDB"];
	        this.sourceRoot = source["sourceRoot"];
	        this.postgresPassword = source["postgresPassword"];
	        this.installDir = source["installDir"];
	        this.dbAction = source["dbAction"];
	        this.restoreSqlPath = source["restoreSqlPath"];
	        this.reinstallExisting = source["reinstallExisting"];
	    }
	}
	export class UninstallOptions {
	    installDir: string;
	    postgresPassword: string;
	    skipDB: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UninstallOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installDir = source["installDir"];
	        this.postgresPassword = source["postgresPassword"];
	        this.skipDB = source["skipDB"];
	    }
	}

}

