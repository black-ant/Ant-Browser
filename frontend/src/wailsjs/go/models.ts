export namespace backend {
	
	export class ActivityEntry {
	    timestamp: number;
	    type: string;
	    level: string;
	    message: string;
	    profileName?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivityEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.profileName = source["profileName"];
	    }
	}
	export class BrowserAccount {
	    accountId: string;
	    accountName: string;
	    platform: string;
	    username: string;
	    email: string;
	    password: string;
	    relatedProfileIds: string[];
	    notes: string;
	    cookies: string;
	    createdAt: string;
	    updatedAt: string;
	    cookieCount: number;
	    cookieEarliestExpiry: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.accountName = source["accountName"];
	        this.platform = source["platform"];
	        this.username = source["username"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.relatedProfileIds = source["relatedProfileIds"];
	        this.notes = source["notes"];
	        this.cookies = source["cookies"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.cookieCount = source["cookieCount"];
	        this.cookieEarliestExpiry = source["cookieEarliestExpiry"];
	    }
	}
	export class BrowserAccountInput {
	    accountName: string;
	    platform: string;
	    username: string;
	    email: string;
	    password: string;
	    relatedProfileIds: string[];
	    notes: string;
	    cookies: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserAccountInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountName = source["accountName"];
	        this.platform = source["platform"];
	        this.username = source["username"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.relatedProfileIds = source["relatedProfileIds"];
	        this.notes = source["notes"];
	        this.cookies = source["cookies"];
	    }
	}
	export class BrowserExtension {
	    extensionId: string;
	    extensionName: string;
	    extensionPath: string;
	    version: string;
	    enabled: boolean;
	    boundProfileIds: string[];
	    sourceType: string;
	    sourceUrl: string;
	    description: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.extensionName = source["extensionName"];
	        this.extensionPath = source["extensionPath"];
	        this.version = source["version"];
	        this.enabled = source["enabled"];
	        this.boundProfileIds = source["boundProfileIds"];
	        this.sourceType = source["sourceType"];
	        this.sourceUrl = source["sourceUrl"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class BrowserExtensionInput {
	    extensionName: string;
	    extensionPath: string;
	    version: string;
	    enabled: boolean;
	    boundProfileIds: string[];
	    sourceType: string;
	    sourceUrl: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionName = source["extensionName"];
	        this.extensionPath = source["extensionPath"];
	        this.version = source["version"];
	        this.enabled = source["enabled"];
	        this.boundProfileIds = source["boundProfileIds"];
	        this.sourceType = source["sourceType"];
	        this.sourceUrl = source["sourceUrl"];
	        this.description = source["description"];
	    }
	}
	export class CookieInfo {
	    name: string;
	    value: string;
	    domain: string;
	    path: string;
	    expires: number;
	    httpOnly: boolean;
	    secure: boolean;
	    sameSite: string;
	
	    static createFrom(source: any = {}) {
	        return new CookieInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.domain = source["domain"];
	        this.path = source["path"];
	        this.expires = source["expires"];
	        this.httpOnly = source["httpOnly"];
	        this.secure = source["secure"];
	        this.sameSite = source["sameSite"];
	    }
	}
	export class MetricSample {
	    timestamp: number;
	    cpuPercent: number;
	    memUsedMB: number;
	    memTotalMB: number;
	    appMemMB: number;
	    runningInstances: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memUsedMB = source["memUsedMB"];
	        this.memTotalMB = source["memTotalMB"];
	        this.appMemMB = source["appMemMB"];
	        this.runningInstances = source["runningInstances"];
	    }
	}
	export class DashboardMetrics {
	    live: MetricSample;
	    history: MetricSample[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.live = this.convertValues(source["live"], MetricSample);
	        this.history = this.convertValues(source["history"], MetricSample);
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
	export class ExtensionValidateResult {
	    valid: boolean;
	    message: string;
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionValidateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.message = source["message"];
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class LicenseStatus {
	    maxLimit: number;
	    usedCount: number;
	    usedKeys: string[];
	
	    static createFrom(source: any = {}) {
	        return new LicenseStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxLimit = source["maxLimit"];
	        this.usedCount = source["usedCount"];
	        this.usedKeys = source["usedKeys"];
	    }
	}
	
	export class ProxyIPHealthResult {
	    proxyId: string;
	    ok: boolean;
	    source: string;
	    error: string;
	    ip: string;
	    fraudScore: number;
	    isResidential: boolean;
	    isBroadcast: boolean;
	    country: string;
	    region: string;
	    city: string;
	    asOrganization: string;
	    rawData: Record<string, any>;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyIPHealthResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.source = source["source"];
	        this.error = source["error"];
	        this.ip = source["ip"];
	        this.fraudScore = source["fraudScore"];
	        this.isResidential = source["isResidential"];
	        this.isBroadcast = source["isBroadcast"];
	        this.country = source["country"];
	        this.region = source["region"];
	        this.city = source["city"];
	        this.asOrganization = source["asOrganization"];
	        this.rawData = source["rawData"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProxyTestResult {
	    proxyId: string;
	    ok: boolean;
	    latencyMs: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class ProxyValidationResult {
	    supported: boolean;
	    errorMsg: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.errorMsg = source["errorMsg"];
	    }
	}
	export class SnapshotInfo {
	    snapshotId: string;
	    profileId: string;
	    name: string;
	    sizeMB: number;
	    createdAt: string;
	    filePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshotId = source["snapshotId"];
	        this.profileId = source["profileId"];
	        this.name = source["name"];
	        this.sizeMB = source["sizeMB"];
	        this.createdAt = source["createdAt"];
	        this.filePath = source["filePath"];
	    }
	}

}

export namespace backup {
	
	export class ManifestEntry {
	    id: string;
	    category: string;
	    entryType: string;
	    required: boolean;
	    archivePath: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.entryType = source["entryType"];
	        this.required = source["required"];
	        this.archivePath = source["archivePath"];
	        this.description = source["description"];
	    }
	}
	export class ManifestAppInfo {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestAppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class Manifest {
	    format: string;
	    manifestVersion: number;
	    createdAt: string;
	    app: ManifestAppInfo;
	    entries: ManifestEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.manifestVersion = source["manifestVersion"];
	        this.createdAt = source["createdAt"];
	        this.app = this.convertValues(source["app"], ManifestAppInfo);
	        this.entries = this.convertValues(source["entries"], ManifestEntry);
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
	
	
	export class ScopeEntry {
	    id: string;
	    category: string;
	    entryType: string;
	    required: boolean;
	    sourcePath: string;
	    archivePath: string;
	    exists: boolean;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScopeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.entryType = source["entryType"];
	        this.required = source["required"];
	        this.sourcePath = source["sourcePath"];
	        this.archivePath = source["archivePath"];
	        this.exists = source["exists"];
	        this.description = source["description"];
	    }
	}
	export class Scope {
	    format: string;
	    manifestVersion: number;
	    appRoot: string;
	    entries: ScopeEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Scope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.manifestVersion = source["manifestVersion"];
	        this.appRoot = source["appRoot"];
	        this.entries = this.convertValues(source["entries"], ScopeEntry);
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
	
	export class CoreExtendedInfo {
	    coreId: string;
	    chromeVersion: string;
	    instanceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new CoreExtendedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.chromeVersion = source["chromeVersion"];
	        this.instanceCount = source["instanceCount"];
	    }
	}
	export class CoreInput {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CoreInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	    }
	}
	export class CoreValidateResult {
	    valid: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CoreValidateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.message = source["message"];
	    }
	}
	export class Group {
	    groupId: string;
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupId = source["groupId"];
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class GroupInput {
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	    }
	}
	export class GroupWithCount {
	    groupId: string;
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	    instanceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupWithCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupId = source["groupId"];
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.instanceCount = source["instanceCount"];
	    }
	}
	export class Profile {
	    profileId: string;
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    proxyBindSourceId: string;
	    proxyBindSourceUrl: string;
	    proxyBindName: string;
	    proxyBindUpdatedAt: string;
	    launchArgs: string[];
	    tags: string[];
	    keywords: string[];
	    groupId: string;
	    launchCode: string;
	    running: boolean;
	    status: string;
	    debugPort: number;
	    debugReady: boolean;
	    pid: number;
	    runtimeWarning: string;
	    lastError: string;
	    createdAt: string;
	    updatedAt: string;
	    lastStartAt: string;
	    lastStopAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.proxyBindSourceId = source["proxyBindSourceId"];
	        this.proxyBindSourceUrl = source["proxyBindSourceUrl"];
	        this.proxyBindName = source["proxyBindName"];
	        this.proxyBindUpdatedAt = source["proxyBindUpdatedAt"];
	        this.launchArgs = source["launchArgs"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.groupId = source["groupId"];
	        this.launchCode = source["launchCode"];
	        this.running = source["running"];
	        this.status = source["status"];
	        this.debugPort = source["debugPort"];
	        this.debugReady = source["debugReady"];
	        this.pid = source["pid"];
	        this.runtimeWarning = source["runtimeWarning"];
	        this.lastError = source["lastError"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastStartAt = source["lastStartAt"];
	        this.lastStopAt = source["lastStopAt"];
	    }
	}
	export class ProfileInput {
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    launchArgs: string[];
	    tags: string[];
	    keywords: string[];
	    groupId: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.launchArgs = source["launchArgs"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.groupId = source["groupId"];
	    }
	}
	export class ProxySource {
	    sourceId: string;
	    sourceUrl: string;
	    sourceName: string;
	    groupName: string;
	    namePrefix: string;
	    dnsServers: string;
	    autoRefresh: boolean;
	    refreshIntervalM: number;
	    importStrategy: string;
	    lastRefreshAt: string;
	    lastRefreshError: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxySource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceName = source["sourceName"];
	        this.groupName = source["groupName"];
	        this.namePrefix = source["namePrefix"];
	        this.dnsServers = source["dnsServers"];
	        this.autoRefresh = source["autoRefresh"];
	        this.refreshIntervalM = source["refreshIntervalM"];
	        this.importStrategy = source["importStrategy"];
	        this.lastRefreshAt = source["lastRefreshAt"];
	        this.lastRefreshError = source["lastRefreshError"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ProxySourceOverride {
	    sourceId: string;
	    nodeKey: string;
	    action: string;
	    customName: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxySourceOverride(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.nodeKey = source["nodeKey"];
	        this.action = source["action"];
	        this.customName = source["customName"];
	    }
	}
	export class Settings {
	    userDataRoot: string;
	    defaultFingerprintArgs: string[];
	    defaultLaunchArgs: string[];
	    defaultProxy: string;
	    startReadyTimeoutMs: number;
	    startStableWindowMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userDataRoot = source["userDataRoot"];
	        this.defaultFingerprintArgs = source["defaultFingerprintArgs"];
	        this.defaultLaunchArgs = source["defaultLaunchArgs"];
	        this.defaultProxy = source["defaultProxy"];
	        this.startReadyTimeoutMs = source["startReadyTimeoutMs"];
	        this.startStableWindowMs = source["startStableWindowMs"];
	    }
	}
	export class Tab {
	    tabId: string;
	    title: string;
	    url: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tabId = source["tabId"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.active = source["active"];
	    }
	}

}

export namespace cdp {
	
	export class ConsoleLog {
	    id: string;
	    type: string;
	    message: string;
	    timestamp: number;
	    stackTrace?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConsoleLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.message = source["message"];
	        this.timestamp = source["timestamp"];
	        this.stackTrace = source["stackTrace"];
	    }
	}
	export class NetworkRequest {
	    requestId: string;
	    url: string;
	    method: string;
	    statusCode: number;
	    statusText: string;
	    type: string;
	    timestamp: number;
	    requestHeaders: Record<string, string>;
	    responseHeaders: Record<string, string>;
	    requestBody: string;
	    responseBody: string;
	    duration: number;
	    size: number;
	    mimeType: string;
	    startTime: number;
	    endTime: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestId = source["requestId"];
	        this.url = source["url"];
	        this.method = source["method"];
	        this.statusCode = source["statusCode"];
	        this.statusText = source["statusText"];
	        this.type = source["type"];
	        this.timestamp = source["timestamp"];
	        this.requestHeaders = source["requestHeaders"];
	        this.responseHeaders = source["responseHeaders"];
	        this.requestBody = source["requestBody"];
	        this.responseBody = source["responseBody"];
	        this.duration = source["duration"];
	        this.size = source["size"];
	        this.mimeType = source["mimeType"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	    }
	}

}

export namespace config {
	
	export class BrowserBookmark {
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class BrowserCore {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserCore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	    }
	}
	export class BrowserProxy {
	    proxyId: string;
	    proxyName: string;
	    proxyConfig: string;
	    dnsServers?: string;
	    groupName?: string;
	    sortOrder?: number;
	    sourceId?: string;
	    sourceUrl?: string;
	    sourceNamePrefix?: string;
	    sourceAutoRefresh?: boolean;
	    sourceRefreshIntervalM?: number;
	    sourceLastRefreshAt?: string;
	    lastLatencyMs: number;
	    lastTestOk: boolean;
	    lastTestedAt: string;
	    lastIPHealthJson?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.proxyName = source["proxyName"];
	        this.proxyConfig = source["proxyConfig"];
	        this.dnsServers = source["dnsServers"];
	        this.groupName = source["groupName"];
	        this.sortOrder = source["sortOrder"];
	        this.sourceId = source["sourceId"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceNamePrefix = source["sourceNamePrefix"];
	        this.sourceAutoRefresh = source["sourceAutoRefresh"];
	        this.sourceRefreshIntervalM = source["sourceRefreshIntervalM"];
	        this.sourceLastRefreshAt = source["sourceLastRefreshAt"];
	        this.lastLatencyMs = source["lastLatencyMs"];
	        this.lastTestOk = source["lastTestOk"];
	        this.lastTestedAt = source["lastTestedAt"];
	        this.lastIPHealthJson = source["lastIPHealthJson"];
	    }
	}

}

export namespace launchcode {
	
	export class LaunchRequestParams {
	    launchArgs: string[];
	    startUrls: string[];
	    skipDefaultStartUrls: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LaunchRequestParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.launchArgs = source["launchArgs"];
	        this.startUrls = source["startUrls"];
	        this.skipDefaultStartUrls = source["skipDefaultStartUrls"];
	    }
	}

}

export namespace logger {
	
	export class MemoryLogEntry {
	    time: string;
	    level: string;
	    component: string;
	    message: string;
	    fields?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new MemoryLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.component = source["component"];
	        this.message = source["message"];
	        this.fields = source["fields"];
	    }
	}
	export class MethodInterceptor {
	
	
	    static createFrom(source: any = {}) {
	        return new MethodInterceptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace usernamescan {
	
	export class BrowserScanOptions {
	    profileId: string;
	    url: string;
	    inputSelector: string;
	    submitSelector: string;
	    resultSelector: string;
	    availableText: string;
	    takenText: string;
	    holdText: string;
	    waitAfterSubmitMs: number;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserScanOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.url = source["url"];
	        this.inputSelector = source["inputSelector"];
	        this.submitSelector = source["submitSelector"];
	        this.resultSelector = source["resultSelector"];
	        this.availableText = source["availableText"];
	        this.takenText = source["takenText"];
	        this.holdText = source["holdText"];
	        this.waitAfterSubmitMs = source["waitAfterSubmitMs"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class GeneratorOptions {
	    sourceText: string;
	    targetLength: number;
	    allowDigits: boolean;
	    maxResults: number;
	
	    static createFrom(source: any = {}) {
	        return new GeneratorOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceText = source["sourceText"];
	        this.targetLength = source["targetLength"];
	        this.allowDigits = source["allowDigits"];
	        this.maxResults = source["maxResults"];
	    }
	}
	export class History {
	    checked: number[];
	    hit: number[];
	    rate: number[];
	
	    static createFrom(source: any = {}) {
	        return new History(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.hit = source["hit"];
	        this.rate = source["rate"];
	    }
	}
	export class LogEntry {
	    level: string;
	    message: string;
	    params: Record<string, any>;
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.message = source["message"];
	        this.params = source["params"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class Result {
	    name: string;
	    status: string;
	    message: string;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	export class Stats {
	    checked: number;
	    hit: number;
	    taken: number;
	    hold: number;
	    error: number;
	    rate: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.hit = source["hit"];
	        this.taken = source["taken"];
	        this.hold = source["hold"];
	        this.error = source["error"];
	        this.rate = source["rate"];
	    }
	}
	export class Snapshot {
	    running: boolean;
	    paused: boolean;
	    provider: string;
	    queueSize: number;
	    nextIndex: number;
	    activeName: string;
	    stats: Stats;
	    history: History;
	    results: Result[];
	    logs: LogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.paused = source["paused"];
	        this.provider = source["provider"];
	        this.queueSize = source["queueSize"];
	        this.nextIndex = source["nextIndex"];
	        this.activeName = source["activeName"];
	        this.stats = this.convertValues(source["stats"], Stats);
	        this.history = this.convertValues(source["history"], History);
	        this.results = this.convertValues(source["results"], Result);
	        this.logs = this.convertValues(source["logs"], LogEntry);
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
	export class StartRequest {
	    seeds: string[];
	    candidates: string[];
	    provider: string;
	    browser: BrowserScanOptions;
	    intervalMs: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new StartRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seeds = source["seeds"];
	        this.candidates = source["candidates"];
	        this.provider = source["provider"];
	        this.browser = this.convertValues(source["browser"], BrowserScanOptions);
	        this.intervalMs = source["intervalMs"];
	        this.limit = source["limit"];
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

