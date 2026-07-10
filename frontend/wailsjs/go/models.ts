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
	export class DatabaseBackupResultDTO {
	    success: boolean;
	    path: string;
	    message: string;
	    serverRestarted: boolean;
	    log?: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseBackupResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.path = source["path"];
	        this.message = source["message"];
	        this.serverRestarted = source["serverRestarted"];
	        this.log = source["log"];
	    }
	}
	export class DbEngineDTO {
	    kind: string;
	    label: string;
	    binDir: string;
	    version: string;
	    isManual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DbEngineDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.binDir = source["binDir"];
	        this.version = source["version"];
	        this.isManual = source["isManual"];
	    }
	}
	export class DbCheckResult {
	    engines: DbEngineDTO[];
	    installed: boolean;
	    installerFound: boolean;
	    installerPath: string;
	    activeEngineKind: string;
	
	    static createFrom(source: any = {}) {
	        return new DbCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engines = this.convertValues(source["engines"], DbEngineDTO);
	        this.installed = source["installed"];
	        this.installerFound = source["installerFound"];
	        this.installerPath = source["installerPath"];
	        this.activeEngineKind = source["activeEngineKind"];
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
	export class NextcloudUpdateCheckDTO {
	    currentVersion: string;
	    remoteVersion: string;
	    updateRequired: boolean;
	    archiveName: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new NextcloudUpdateCheckDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.remoteVersion = source["remoteVersion"];
	        this.updateRequired = source["updateRequired"];
	        this.archiveName = source["archiveName"];
	        this.message = source["message"];
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
	export class PostgresInstallerDTO {
	    path: string;
	    label: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new PostgresInstallerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.version = source["version"];
	    }
	}
	export class ServerSessionDTO {
	    userId: number;
	    login: string;
	    username: string;
	    ip: string;
	    company: string;
	    module: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerSessionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userId = source["userId"];
	        this.login = source["login"];
	        this.username = source["username"];
	        this.ip = source["ip"];
	        this.company = source["company"];
	        this.module = source["module"];
	    }
	}
	export class ServerStatusDTO {
	    taskExists: boolean;
	    running: boolean;
	    raw?: string;
	    networkPort: number;
	    managementPort: number;
	    propertiesPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskExists = source["taskExists"];
	        this.running = source["running"];
	        this.raw = source["raw"];
	        this.networkPort = source["networkPort"];
	        this.managementPort = source["managementPort"];
	        this.propertiesPath = source["propertiesPath"];
	    }
	}
	export class ServerInfoDTO {
	    status: ServerStatusDTO;
	    sessions: ServerSessionDTO[];
	    version?: string;
	    sessionCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], ServerStatusDTO);
	        this.sessions = this.convertValues(source["sessions"], ServerSessionDTO);
	        this.version = source["version"];
	        this.sessionCount = source["sessionCount"];
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
	
	
	export class ServerUpdateResultDTO {
	    success: boolean;
	    message: string;
	    archivePath?: string;
	    log?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerUpdateResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.archivePath = source["archivePath"];
	        this.log = source["log"];
	    }
	}
	export class ServerUpdateTargetDTO {
	    platform: string;
	    archiveName: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerUpdateTargetDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.archiveName = source["archiveName"];
	    }
	}
	export class SourceValidationResult {
	    valid: boolean;
	    missing: string[];
	
	    static createFrom(source: any = {}) {
	        return new SourceValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.missing = source["missing"];
	    }
	}
	export class StartInstallOptions {
	    installServer: boolean;
	    installClients: boolean;
	    installDB: boolean;
	    sourceRoot: string;
	    dbEngine: string;
	    postgresUser: string;
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
	        this.dbEngine = source["dbEngine"];
	        this.postgresUser = source["postgresUser"];
	        this.postgresPassword = source["postgresPassword"];
	        this.installDir = source["installDir"];
	        this.dbAction = source["dbAction"];
	        this.restoreSqlPath = source["restoreSqlPath"];
	        this.reinstallExisting = source["reinstallExisting"];
	    }
	}
	export class UninstallOptions {
	    installDir: string;
	    dbEngine: string;
	    postgresUser: string;
	    postgresPassword: string;
	    skipDB: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UninstallOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installDir = source["installDir"];
	        this.dbEngine = source["dbEngine"];
	        this.postgresUser = source["postgresUser"];
	        this.postgresPassword = source["postgresPassword"];
	        this.skipDB = source["skipDB"];
	    }
	}
	export class UpdateSourceConfigDTO {
	    baseURL: string;
	    updatesFolder: string;
	    configPath: string;
	    configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateSourceConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseURL = source["baseURL"];
	        this.updatesFolder = source["updatesFolder"];
	        this.configPath = source["configPath"];
	        this.configured = source["configured"];
	    }
	}

}

