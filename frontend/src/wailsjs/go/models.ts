export namespace appupdate {

	export class State {
	    kind: string;
	    status: string;
	    localAppVersion: string;
	    remoteAppVersion: string;
	    minimumRuntimeResourceVersion: string;
	    manifestSource: string;
	    manifestUrl: string;
	    payloadUrl: string;
	    target: string;
	    notes: string;
	    errorCode: string;
	    errorMessage: string;
	    details: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new State(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.localAppVersion = source["localAppVersion"];
	        this.remoteAppVersion = source["remoteAppVersion"];
	        this.minimumRuntimeResourceVersion = source["minimumRuntimeResourceVersion"];
	        this.manifestSource = source["manifestSource"];
	        this.manifestUrl = source["manifestUrl"];
	        this.payloadUrl = source["payloadUrl"];
	        this.target = source["target"];
	        this.notes = source["notes"];
	        this.errorCode = source["errorCode"];
	        this.errorMessage = source["errorMessage"];
	        this.details = source["details"];
	    }
	}

}

export namespace authsession {

	export class Session {
	    accessToken: string;
	    rememberMe: boolean;

	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessToken = source["accessToken"];
	        this.rememberMe = source["rememberMe"];
	    }
	}

}

export namespace backend {

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
	export class DesktopAuthRole {
	    code: string;
	    name: string;

	    static createFrom(source: any = {}) {
	        return new DesktopAuthRole(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	    }
	}
	export class DesktopAuthUser {
	    id: string;
	    displayName: string;
	    username: string;

	    static createFrom(source: any = {}) {
	        return new DesktopAuthUser(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.username = source["username"];
	    }
	}
	export class DesktopAuthProfile {
	    user: DesktopAuthUser;
	    roles: DesktopAuthRole[];
	    dataScope: string;

	    static createFrom(source: any = {}) {
	        return new DesktopAuthProfile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = this.convertValues(source["user"], DesktopAuthUser);
	        this.roles = this.convertValues(source["roles"], DesktopAuthRole);
	        this.dataScope = source["dataScope"];
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


	export class DesktopServerConnection {
	    serverOrigin: string;
	    source: string;
	    configPath: string;

	    static createFrom(source: any = {}) {
	        return new DesktopServerConnection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverOrigin = source["serverOrigin"];
	        this.source = source["source"];
	        this.configPath = source["configPath"];
	    }
	}
	export class DesktopSharedLoginDetail {
	    shopId: string;
	    shopName: string;
	    platformCode: string;
	    sharedLoginStatus: string;
	    sharedLoginStatusLabel: string;

	    static createFrom(source: any = {}) {
	        return new DesktopSharedLoginDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shopId = source["shopId"];
	        this.shopName = source["shopName"];
	        this.platformCode = source["platformCode"];
	        this.sharedLoginStatus = source["sharedLoginStatus"];
	        this.sharedLoginStatusLabel = source["sharedLoginStatusLabel"];
	    }
	}
	export class DesktopSharedLoginBindSession {
	    bindSessionId: string;
	    traceId: string;
	    shopId: string;
	    shopName: string;
	    sessionType: string;
	    status: string;
	    statusLabel: string;
	    message: string;
	    manualActionRequired: boolean;
	    lastObservedUrl: string;
	    startedAt: string;
	    expiresAt: string;
	    completedAt: string;
	    updatedAt: string;
	    challengeType: string;

	    static createFrom(source: any = {}) {
	        return new DesktopSharedLoginBindSession(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bindSessionId = source["bindSessionId"];
	        this.traceId = source["traceId"];
	        this.shopId = source["shopId"];
	        this.shopName = source["shopName"];
	        this.sessionType = source["sessionType"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	        this.message = source["message"];
	        this.manualActionRequired = source["manualActionRequired"];
	        this.lastObservedUrl = source["lastObservedUrl"];
	        this.startedAt = source["startedAt"];
	        this.expiresAt = source["expiresAt"];
	        this.completedAt = source["completedAt"];
	        this.updatedAt = source["updatedAt"];
	        this.challengeType = source["challengeType"];
	    }
	}
	export class DesktopSharedLoginActionResult {
	    bindSession: DesktopSharedLoginBindSession;
	    detail: DesktopSharedLoginDetail;

	    static createFrom(source: any = {}) {
	        return new DesktopSharedLoginActionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bindSession = this.convertValues(source["bindSession"], DesktopSharedLoginBindSession);
	        this.detail = this.convertValues(source["detail"], DesktopSharedLoginDetail);
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
	export class ManagedProfileUpsertInput {
	    profileId: string;
	    shopId: string;
	    platformCode: string;
	    profileName: string;
	    managedMode: boolean;
	    userDataDir: string;

	    static createFrom(source: any = {}) {
	        return new ManagedProfileUpsertInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.shopId = source["shopId"];
	        this.platformCode = source["platformCode"];
	        this.profileName = source["profileName"];
	        this.managedMode = source["managedMode"];
	        this.userDataDir = source["userDataDir"];
	    }
	}
	export class ManagedProfileUpsertResult {
	    ProfileID: string;
	    Updated: boolean;

	    static createFrom(source: any = {}) {
	        return new ManagedProfileUpsertResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ProfileID = source["ProfileID"];
	        this.Updated = source["Updated"];
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

export namespace release {

	export class FailureItem {
	    code: string;
	    severity: string;
	    message: string;
	    repairable: boolean;
	    recommendedAction?: string;
	    details?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new FailureItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	        this.repairable = source["repairable"];
	        this.recommendedAction = source["recommendedAction"];
	        this.details = source["details"];
	    }
	}
	export class CheckResult {
	    state: string;
	    items: FailureItem[];

	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.items = this.convertValues(source["items"], FailureItem);
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

	export class UpdateState {
	    kind: string;
	    localAppVersion: string;
	    remoteAppVersion: string;
	    resourceVersion: string;
	    manifestSource?: string;
	    manifestUrl?: string;

	    static createFrom(source: any = {}) {
	        return new UpdateState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.localAppVersion = source["localAppVersion"];
	        this.remoteAppVersion = source["remoteAppVersion"];
	        this.resourceVersion = source["resourceVersion"];
	        this.manifestSource = source["manifestSource"];
	        this.manifestUrl = source["manifestUrl"];
	    }
	}

}

export namespace workspace {

	export class OpenShopResult {
	    shopId: string;
	    profileId: string;
	    instanceId: string;
	    currentUrl: string;
	    pageTitle: string;
	    success: boolean;
	    code: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new OpenShopResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shopId = source["shopId"];
	        this.profileId = source["profileId"];
	        this.instanceId = source["instanceId"];
	        this.currentUrl = source["currentUrl"];
	        this.pageTitle = source["pageTitle"];
	        this.success = source["success"];
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class OperationTaskQuery {
	    Limit: number;
	    Status: string;
	    ShopID: string;
	    TaskType: string;

	    static createFrom(source: any = {}) {
	        return new OperationTaskQuery(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Limit = source["Limit"];
	        this.Status = source["Status"];
	        this.ShopID = source["ShopID"];
	        this.TaskType = source["TaskType"];
	    }
	}
	export class OperationTaskRecord {
	    taskId: string;
	    shopId: string;
	    shopName: string;
	    taskType: string;
	    title: string;
	    status: string;
	    blockedReason: string;
	    failureMessage: string;
	    updatedAt: string;
	    runId: string;
	    failureCode: string;

	    static createFrom(source: any = {}) {
	        return new OperationTaskRecord(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.shopId = source["shopId"];
	        this.shopName = source["shopName"];
	        this.taskType = source["taskType"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.blockedReason = source["blockedReason"];
	        this.failureMessage = source["failureMessage"];
	        this.updatedAt = source["updatedAt"];
	        this.runId = source["runId"];
	        this.failureCode = source["failureCode"];
	    }
	}
	export class OperationTasksPayload {
	    items: OperationTaskRecord[];
	    total: number;

	    static createFrom(source: any = {}) {
	        return new OperationTasksPayload(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], OperationTaskRecord);
	        this.total = source["total"];
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
	export class RunEvent {
	    eventId: string;
	    stage: string;
	    message: string;
	    details?: Record<string, any>;
	    createdAt: string;

	    static createFrom(source: any = {}) {
	        return new RunEvent(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventId = source["eventId"];
	        this.stage = source["stage"];
	        this.message = source["message"];
	        this.details = source["details"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class RunEventsPayload {
	    runId: string;
	    items: RunEvent[];
	    total: number;

	    static createFrom(source: any = {}) {
	        return new RunEventsPayload(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.items = this.convertValues(source["items"], RunEvent);
	        this.total = source["total"];
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
	export class RunRuntime {
	    pid: number;
	    debugPort: number;
	    currentUrl: string;
	    pageTitle: string;
	    targetUrl: string;

	    static createFrom(source: any = {}) {
	        return new RunRuntime(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.debugPort = source["debugPort"];
	        this.currentUrl = source["currentUrl"];
	        this.pageTitle = source["pageTitle"];
	        this.targetUrl = source["targetUrl"];
	    }
	}
	export class RunRecord {
	    runId: string;
	    taskId: string;
	    shopId: string;
	    taskType: string;
	    status: string;
	    statusLabel: string;
	    startedAt: string;
	    finishedAt: string;
	    profileId: string;
	    runtime?: RunRuntime;
	    bindSessionId: string;
	    manualActionRequired: boolean;
	    challengeType: string;
	    failureCode: string;
	    failureMessage: string;

	    static createFrom(source: any = {}) {
	        return new RunRecord(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.taskId = source["taskId"];
	        this.shopId = source["shopId"];
	        this.taskType = source["taskType"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.profileId = source["profileId"];
	        this.runtime = this.convertValues(source["runtime"], RunRuntime);
	        this.bindSessionId = source["bindSessionId"];
	        this.manualActionRequired = source["manualActionRequired"];
	        this.challengeType = source["challengeType"];
	        this.failureCode = source["failureCode"];
	        this.failureMessage = source["failureMessage"];
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
	export class ShopRunEvidence {
	    latestOpen?: RunRecord;
	    latestCredential?: RunRecord;
	    latestValidation?: RunRecord;
	    latestFailure?: RunRecord;
	    activeRun?: RunRecord;

	    static createFrom(source: any = {}) {
	        return new ShopRunEvidence(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latestOpen = this.convertValues(source["latestOpen"], RunRecord);
	        this.latestCredential = this.convertValues(source["latestCredential"], RunRecord);
	        this.latestValidation = this.convertValues(source["latestValidation"], RunRecord);
	        this.latestFailure = this.convertValues(source["latestFailure"], RunRecord);
	        this.activeRun = this.convertValues(source["activeRun"], RunRecord);
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
	export class RunEvidenceIndex {
	    byShop: Record<string, ShopRunEvidence>;

	    static createFrom(source: any = {}) {
	        return new RunEvidenceIndex(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.byShop = this.convertValues(source["byShop"], ShopRunEvidence, true);
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
	export class RunQuery {
	    Limit: number;
	    Status: string;
	    ShopID: string;
	    FailureCode: string;

	    static createFrom(source: any = {}) {
	        return new RunQuery(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Limit = source["Limit"];
	        this.Status = source["Status"];
	        this.ShopID = source["ShopID"];
	        this.FailureCode = source["FailureCode"];
	    }
	}


	export class RunsPayload {
	    items: RunRecord[];
	    total: number;

	    static createFrom(source: any = {}) {
	        return new RunsPayload(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], RunRecord);
	        this.total = source["total"];
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
	export class SessionStorageEntry {
	    origin: string;
	    scope: string;
	    localStorage: Record<string, string>;
	    sessionStorage: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new SessionStorageEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.origin = source["origin"];
	        this.scope = source["scope"];
	        this.localStorage = source["localStorage"];
	        this.sessionStorage = source["sessionStorage"];
	    }
	}
	export class SessionCookie {
	    name: string;
	    value: string;
	    domain: string;
	    path: string;
	    expires: number;
	    httpOnly: boolean;
	    secure: boolean;
	    sameSite: string;
	    url: string;

	    static createFrom(source: any = {}) {
	        return new SessionCookie(source);
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
	        this.url = source["url"];
	    }
	}
	export class SessionBundle {
	    platformCode: string;
	    capturedAt: string;
	    captureStartedAt: string;
	    lastObservedUrl: string;
	    userAgent: string;
	    cookies: SessionCookie[];
	    storages: SessionStorageEntry[];

	    static createFrom(source: any = {}) {
	        return new SessionBundle(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platformCode = source["platformCode"];
	        this.capturedAt = source["capturedAt"];
	        this.captureStartedAt = source["captureStartedAt"];
	        this.lastObservedUrl = source["lastObservedUrl"];
	        this.userAgent = source["userAgent"];
	        this.cookies = this.convertValues(source["cookies"], SessionCookie);
	        this.storages = this.convertValues(source["storages"], SessionStorageEntry);
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


	export class ShopInstanceProjection {
	    shopId: string;
	    shopName: string;
	    platformCode: string;
	    profileId: string;
	    instanceId: string;
	    sharedLoginStatus: string;
	    sharedLoginStatusLabel: string;
	    instanceRunning: boolean;
	    profileExists: boolean;
	    reclaimPending: boolean;
	    coreReady: boolean;
	    lastValidatedAt?: string;
	    lastOpenedAt?: string;
	    lastOpenFailureCode?: string;
	    lastOpenFailureMessage?: string;
	    lastOpenFailedAt?: string;

	    static createFrom(source: any = {}) {
	        return new ShopInstanceProjection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shopId = source["shopId"];
	        this.shopName = source["shopName"];
	        this.platformCode = source["platformCode"];
	        this.profileId = source["profileId"];
	        this.instanceId = source["instanceId"];
	        this.sharedLoginStatus = source["sharedLoginStatus"];
	        this.sharedLoginStatusLabel = source["sharedLoginStatusLabel"];
	        this.instanceRunning = source["instanceRunning"];
	        this.profileExists = source["profileExists"];
	        this.reclaimPending = source["reclaimPending"];
	        this.coreReady = source["coreReady"];
	        this.lastValidatedAt = source["lastValidatedAt"];
	        this.lastOpenedAt = source["lastOpenedAt"];
	        this.lastOpenFailureCode = source["lastOpenFailureCode"];
	        this.lastOpenFailureMessage = source["lastOpenFailureMessage"];
	        this.lastOpenFailedAt = source["lastOpenFailedAt"];
	    }
	}
	export class ShopProfileRecord {
	    shopId: string;
	    shopName: string;
	    asmShopId: string;
	    shopCode: string;
	    shopAlias: string;
	    fullShopName: string;
	    platformCode: string;
	    platformName: string;
	    platformSubtype: string;
	    shopStatusCode: number;
	    shopStatus: string;
	    asmStatus: string;
	    authorizationStatus: string;
	    authorizationStatusLabel: string;
	    ownerName: string;
	    operatorName: string;
	    operatorUsername: string;
	    businessManagerName: string;
	    businessManagerUsername: string;
	    department: string;
	    subCompanyName: string;
	    shopUrl: string;
	    shopEmail: string;
	    shopPhone: string;
	    legalRepName: string;
	    businessLicense: string;
	    unifiedSocialCode: string;
	    registeredAddress: string;
	    categoryIds: string[];
	    categoryNames: string[];
	    brandName: string;
	    brandIds: string[];
	    advancedMember: number;
	    advancedMemberName: string;
	    trustPassExpireAt: string;
	    jstShopCount: number;
	    jstShopSummary: string;
	    mabangShopCount: number;
	    mabangShopSummary: string;
	    erpShopCount: number;
	    erpShopSummary: string;
	    abnormalCount: number;
	    abnormalSummary: string;
	    tableSource: string;
	    isPush: number;
	    mainCategory: string;
	    dataCompleteness: string;
	    sourceCreatedAt: string;
	    sourceUpdatedAt: string;
	    lastSyncedAt: string;
	    source: string;

	    static createFrom(source: any = {}) {
	        return new ShopProfileRecord(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shopId = source["shopId"];
	        this.shopName = source["shopName"];
	        this.asmShopId = source["asmShopId"];
	        this.shopCode = source["shopCode"];
	        this.shopAlias = source["shopAlias"];
	        this.fullShopName = source["fullShopName"];
	        this.platformCode = source["platformCode"];
	        this.platformName = source["platformName"];
	        this.platformSubtype = source["platformSubtype"];
	        this.shopStatusCode = source["shopStatusCode"];
	        this.shopStatus = source["shopStatus"];
	        this.asmStatus = source["asmStatus"];
	        this.authorizationStatus = source["authorizationStatus"];
	        this.authorizationStatusLabel = source["authorizationStatusLabel"];
	        this.ownerName = source["ownerName"];
	        this.operatorName = source["operatorName"];
	        this.operatorUsername = source["operatorUsername"];
	        this.businessManagerName = source["businessManagerName"];
	        this.businessManagerUsername = source["businessManagerUsername"];
	        this.department = source["department"];
	        this.subCompanyName = source["subCompanyName"];
	        this.shopUrl = source["shopUrl"];
	        this.shopEmail = source["shopEmail"];
	        this.shopPhone = source["shopPhone"];
	        this.legalRepName = source["legalRepName"];
	        this.businessLicense = source["businessLicense"];
	        this.unifiedSocialCode = source["unifiedSocialCode"];
	        this.registeredAddress = source["registeredAddress"];
	        this.categoryIds = source["categoryIds"];
	        this.categoryNames = source["categoryNames"];
	        this.brandName = source["brandName"];
	        this.brandIds = source["brandIds"];
	        this.advancedMember = source["advancedMember"];
	        this.advancedMemberName = source["advancedMemberName"];
	        this.trustPassExpireAt = source["trustPassExpireAt"];
	        this.jstShopCount = source["jstShopCount"];
	        this.jstShopSummary = source["jstShopSummary"];
	        this.mabangShopCount = source["mabangShopCount"];
	        this.mabangShopSummary = source["mabangShopSummary"];
	        this.erpShopCount = source["erpShopCount"];
	        this.erpShopSummary = source["erpShopSummary"];
	        this.abnormalCount = source["abnormalCount"];
	        this.abnormalSummary = source["abnormalSummary"];
	        this.tableSource = source["tableSource"];
	        this.isPush = source["isPush"];
	        this.mainCategory = source["mainCategory"];
	        this.dataCompleteness = source["dataCompleteness"];
	        this.sourceCreatedAt = source["sourceCreatedAt"];
	        this.sourceUpdatedAt = source["sourceUpdatedAt"];
	        this.lastSyncedAt = source["lastSyncedAt"];
	        this.source = source["source"];
	    }
	}

	export class WorkspaceSummary {
	    status: string;
	    agentStatus: string;
	    sessionReady: boolean;
	    serverReachable: boolean;
	    antRuntimeReachable: boolean;
	    activeRunCount: number;
	    deviceId: string;
	    deviceStatus: string;

	    static createFrom(source: any = {}) {
	        return new WorkspaceSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.agentStatus = source["agentStatus"];
	        this.sessionReady = source["sessionReady"];
	        this.serverReachable = source["serverReachable"];
	        this.antRuntimeReachable = source["antRuntimeReachable"];
	        this.activeRunCount = source["activeRunCount"];
	        this.deviceId = source["deviceId"];
	        this.deviceStatus = source["deviceStatus"];
	    }
	}

}
