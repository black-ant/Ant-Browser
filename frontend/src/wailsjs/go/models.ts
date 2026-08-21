export namespace automation {
	
	export class ScriptPublicAPIVariable {
	    name: string;
	    defaultValue: string;
	    description: string;
	    required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScriptPublicAPIVariable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.defaultValue = source["defaultValue"];
	        this.description = source["description"];
	        this.required = source["required"];
	    }
	}
	export class ScriptPublicAPIConfig {
	    enabled: boolean;
	    method: string;
	    path: string;
	    requestMode: string;
	    responseMode: string;
	    timeoutMs: number;
	    requestBodyText: string;
	    responseBodyText: string;
	    variables: ScriptPublicAPIVariable[];
	
	    static createFrom(source: any = {}) {
	        return new ScriptPublicAPIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.requestMode = source["requestMode"];
	        this.responseMode = source["responseMode"];
	        this.timeoutMs = source["timeoutMs"];
	        this.requestBodyText = source["requestBodyText"];
	        this.responseBodyText = source["responseBodyText"];
	        this.variables = this.convertValues(source["variables"], ScriptPublicAPIVariable);
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
	
	export class ScriptSource {
	    type: string;
	    uri: string;
	    ref: string;
	    path: string;
	    importedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.uri = source["uri"];
	        this.ref = source["ref"];
	        this.path = source["path"];
	        this.importedAt = source["importedAt"];
	    }
	}
	export class ScriptTargetSelector {
	    code: string;
	    profileId: string;
	    profileName: string;
	    groupId: string;
	    keywords: string[];
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScriptTargetSelector(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.groupId = source["groupId"];
	        this.keywords = source["keywords"];
	        this.tags = source["tags"];
	    }
	}
	export class ScriptTargetConfig {
	    mode: string;
	    selector: ScriptTargetSelector;
	    templateSelector: ScriptTargetSelector;
	    createNameTemplate: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptTargetConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.selector = this.convertValues(source["selector"], ScriptTargetSelector);
	        this.templateSelector = this.convertValues(source["templateSelector"], ScriptTargetSelector);
	        this.createNameTemplate = source["createNameTemplate"];
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
	export class ScriptRecord {
	    packageFormat: string;
	    manifestVersion: number;
	    id: string;
	    name: string;
	    description: string;
	    type: string;
	    status: string;
	    entryFile: string;
	    tags: string[];
	    selectorText: string;
	    paramsText: string;
	    scriptText: string;
	    notes: string;
	    targetConfig: ScriptTargetConfig;
	    publicAPI: ScriptPublicAPIConfig;
	    source: ScriptSource;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageFormat = source["packageFormat"];
	        this.manifestVersion = source["manifestVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.entryFile = source["entryFile"];
	        this.tags = source["tags"];
	        this.selectorText = source["selectorText"];
	        this.paramsText = source["paramsText"];
	        this.scriptText = source["scriptText"];
	        this.notes = source["notes"];
	        this.targetConfig = this.convertValues(source["targetConfig"], ScriptTargetConfig);
	        this.publicAPI = this.convertValues(source["publicAPI"], ScriptPublicAPIConfig);
	        this.source = this.convertValues(source["source"], ScriptSource);
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
	export class ScriptRunRecord {
	    id: string;
	    scriptId: string;
	    scriptName: string;
	    scriptType: string;
	    status: string;
	    summary: string;
	    error: string;
	    resultText: string;
	    logText: string;
	    startedAt: string;
	    finishedAt: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ScriptRunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scriptId = source["scriptId"];
	        this.scriptName = source["scriptName"];
	        this.scriptType = source["scriptType"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.error = source["error"];
	        this.resultText = source["resultText"];
	        this.logText = source["logText"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class ScriptRunRequest {
	    scriptId: string;
	    selectorText: string;
	    targetMode?: string;
	    targetInput?: any;
	    paramsText: string;
	    useScriptSelector: boolean;
	    useScriptParams: boolean;
	    timeoutMs?: number;
	
	    static createFrom(source: any = {}) {
	        return new ScriptRunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scriptId = source["scriptId"];
	        this.selectorText = source["selectorText"];
	        this.targetMode = source["targetMode"];
	        this.targetInput = source["targetInput"];
	        this.paramsText = source["paramsText"];
	        this.useScriptSelector = source["useScriptSelector"];
	        this.useScriptParams = source["useScriptParams"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	
	

}

export namespace backend {
	
	export class AutomationScriptImportIssue {
	    path: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptImportIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.message = source["message"];
	    }
	}
	export class AutomationScriptBatchImportResult {
	    imported: automation.ScriptRecord[];
	    failed: AutomationScriptImportIssue[];
	    scanned: number;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptBatchImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = this.convertValues(source["imported"], automation.ScriptRecord);
	        this.failed = this.convertValues(source["failed"], AutomationScriptImportIssue);
	        this.scanned = source["scanned"];
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
	
	export class AutomationScriptPublicAPIInvokeInput {
	    url: string;
	    method: string;
	    bodyText: string;
	    apiKey: string;
	    authHeader: string;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptPublicAPIInvokeInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.method = source["method"];
	        this.bodyText = source["bodyText"];
	        this.apiKey = source["apiKey"];
	        this.authHeader = source["authHeader"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class AutomationScriptPublicAPIInvokeResult {
	    ok: boolean;
	    status: number;
	    statusText: string;
	    bodyText: string;
	    bodyJson: any;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptPublicAPIInvokeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.bodyText = source["bodyText"];
	        this.bodyJson = source["bodyJson"];
	    }
	}
	export class BookmarkSyncResult {
	    total: number;
	    synced: number;
	    skipped: number;
	    failed: number;
	    skippedList: string[];
	    failedList: string[];
	
	    static createFrom(source: any = {}) {
	        return new BookmarkSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.synced = source["synced"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.skippedList = source["skippedList"];
	        this.failedList = source["failedList"];
	    }
	}
	export class BrowserCoreBackendOption {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserCoreBackendOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class BrowserExtensionManualDownloadFile {
	    fileName: string;
	    filePath: string;
	    sizeBytes: number;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionManualDownloadFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.sizeBytes = source["sizeBytes"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class BrowserExtensionManualInstallGuide {
	    extensionId: string;
	    storeUrl: string;
	    downloadUrl: string;
	    downloadDir: string;
	    fileName: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionManualInstallGuide(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.storeUrl = source["storeUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.downloadDir = source["downloadDir"];
	        this.fileName = source["fileName"];
	    }
	}
	export class BrowserExtensionWebStoreRequest {
	    query: string;
	    useProxy: boolean;
	    proxyConfig: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionWebStoreRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.useProxy = source["useProxy"];
	        this.proxyConfig = source["proxyConfig"];
	    }
	}
	export class BrowserFingerprintCapabilityRow {
	    capability: string;
	    status: string;
	    inputArg: string;
	    runtimeArg: string;
	    action: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintCapabilityRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capability = source["capability"];
	        this.status = source["status"];
	        this.inputArg = source["inputArg"];
	        this.runtimeArg = source["runtimeArg"];
	        this.action = source["action"];
	        this.note = source["note"];
	    }
	}
	export class BrowserFingerprintCapabilityReport {
	    profileId: string;
	    coreId: string;
	    coreName: string;
	    coreBackend: string;
	    chromeVersion: string;
	    chromeMajor: number;
	    versionStatus: string;
	    rawArgs: string[];
	    launchArgs: string[];
	    rows: BrowserFingerprintCapabilityRow[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintCapabilityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.coreBackend = source["coreBackend"];
	        this.chromeVersion = source["chromeVersion"];
	        this.chromeMajor = source["chromeMajor"];
	        this.versionStatus = source["versionStatus"];
	        this.rawArgs = source["rawArgs"];
	        this.launchArgs = source["launchArgs"];
	        this.rows = this.convertValues(source["rows"], BrowserFingerprintCapabilityRow);
	        this.warnings = source["warnings"];
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
	
	export class BrowserFingerprintExpectedInfo {
	    language: string;
	    acceptLanguage: string;
	    timezone: string;
	    hardwareConcurrency: string;
	    deviceMemory: string;
	    colorDepth: string;
	    touchPoints: string;
	    windowSize: string;
	    brand: string;
	    brandVersion: string;
	    platform: string;
	    platformVersion: string;
	    seed: string;
	    disableSpoofing: string;
	    webrtcPolicy: string;
	    doNotTrack: string;
	    mediaDevices: string;
	    canvasNoise: string;
	    audioNoise: string;
	    clientRectsNoise: string;
	    fontList: string;
	    webglVendor: string;
	    webglRenderer: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintExpectedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.acceptLanguage = source["acceptLanguage"];
	        this.timezone = source["timezone"];
	        this.hardwareConcurrency = source["hardwareConcurrency"];
	        this.deviceMemory = source["deviceMemory"];
	        this.colorDepth = source["colorDepth"];
	        this.touchPoints = source["touchPoints"];
	        this.windowSize = source["windowSize"];
	        this.brand = source["brand"];
	        this.brandVersion = source["brandVersion"];
	        this.platform = source["platform"];
	        this.platformVersion = source["platformVersion"];
	        this.seed = source["seed"];
	        this.disableSpoofing = source["disableSpoofing"];
	        this.webrtcPolicy = source["webrtcPolicy"];
	        this.doNotTrack = source["doNotTrack"];
	        this.mediaDevices = source["mediaDevices"];
	        this.canvasNoise = source["canvasNoise"];
	        this.audioNoise = source["audioNoise"];
	        this.clientRectsNoise = source["clientRectsNoise"];
	        this.fontList = source["fontList"];
	        this.webglVendor = source["webglVendor"];
	        this.webglRenderer = source["webglRenderer"];
	    }
	}
	export class BrowserFingerprintRuntimeInfo {
	    language: string;
	    languages: string[];
	    timezone: string;
	    hardwareConcurrency: number;
	    deviceMemory: number;
	    maxTouchPoints: number;
	    doNotTrack: string;
	    mediaDeviceCount: number;
	    platform: string;
	    userAgent: string;
	    userAgentData: string;
	    webdriver: boolean;
	    screenWidth: number;
	    screenHeight: number;
	    colorDepth: number;
	    innerWidth: number;
	    innerHeight: number;
	    outerWidth: number;
	    outerHeight: number;
	    devicePixelRatio: number;
	    webglVendor: string;
	    webglRenderer: string;
	    canvasHash: string;
	    audioHash: string;
	    clientRectsHash: string;
	    plugins: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintRuntimeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.languages = source["languages"];
	        this.timezone = source["timezone"];
	        this.hardwareConcurrency = source["hardwareConcurrency"];
	        this.deviceMemory = source["deviceMemory"];
	        this.maxTouchPoints = source["maxTouchPoints"];
	        this.doNotTrack = source["doNotTrack"];
	        this.mediaDeviceCount = source["mediaDeviceCount"];
	        this.platform = source["platform"];
	        this.userAgent = source["userAgent"];
	        this.userAgentData = source["userAgentData"];
	        this.webdriver = source["webdriver"];
	        this.screenWidth = source["screenWidth"];
	        this.screenHeight = source["screenHeight"];
	        this.colorDepth = source["colorDepth"];
	        this.innerWidth = source["innerWidth"];
	        this.innerHeight = source["innerHeight"];
	        this.outerWidth = source["outerWidth"];
	        this.outerHeight = source["outerHeight"];
	        this.devicePixelRatio = source["devicePixelRatio"];
	        this.webglVendor = source["webglVendor"];
	        this.webglRenderer = source["webglRenderer"];
	        this.canvasHash = source["canvasHash"];
	        this.audioHash = source["audioHash"];
	        this.clientRectsHash = source["clientRectsHash"];
	        this.plugins = source["plugins"];
	    }
	}
	export class BrowserFingerprintCheckResult {
	    profileId: string;
	    runtime: BrowserFingerprintRuntimeInfo;
	    expected: BrowserFingerprintExpectedInfo;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.runtime = this.convertValues(source["runtime"], BrowserFingerprintRuntimeInfo);
	        this.expected = this.convertValues(source["expected"], BrowserFingerprintExpectedInfo);
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
	export class ProfilePackageExportResult {
	    cancelled: boolean;
	    zipPath: string;
	    profileCount: number;
	    fileCount: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.zipPath = source["zipPath"];
	        this.profileCount = source["profileCount"];
	        this.fileCount = source["fileCount"];
	        this.message = source["message"];
	    }
	}
	export class ProfilePackageImportResult {
	    cancelled: boolean;
	    importedCount: number;
	    profileMappings: Record<string, string>;
	    warnings: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.importedCount = source["importedCount"];
	        this.profileMappings = source["profileMappings"];
	        this.warnings = source["warnings"];
	        this.message = source["message"];
	    }
	}
	export class ProxyBridgeWarmupResult {
	    proxyId: string;
	    ok: boolean;
	    engine: string;
	    socksUrl: string;
	    latencyMs: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyBridgeWarmupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.engine = source["engine"];
	        this.socksUrl = source["socksUrl"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class ProxyBrowserProbeRequest {
	    proxyId: string;
	    urls: string[];
	    concurrency: number;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ProxyBrowserProbeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.urls = source["urls"];
	        this.concurrency = source["concurrency"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class ProxyBrowserProbeResult {
	    proxyId: string;
	    ok: boolean;
	    totalMs: number;
	    averageMs: number;
	    p95Ms: number;
	    bytes: number;
	    completed: number;
	    failed: number;
	    concurrency: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyBrowserProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.totalMs = source["totalMs"];
	        this.averageMs = source["averageMs"];
	        this.p95Ms = source["p95Ms"];
	        this.bytes = source["bytes"];
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	        this.concurrency = source["concurrency"];
	        this.error = source["error"];
	    }
	}
	export class ProxyCoreDownloadInfoResult {
	    core: string;
	    goos: string;
	    goarch: string;
	    version: string;
	    repo: string;
	    releaseUrl: string;
	    downloadUrl: string;
	    assetName: string;
	    installDir: string;
	    binaryName: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCoreDownloadInfoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.version = source["version"];
	        this.repo = source["repo"];
	        this.releaseUrl = source["releaseUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.assetName = source["assetName"];
	        this.installDir = source["installDir"];
	        this.binaryName = source["binaryName"];
	        this.message = source["message"];
	    }
	}
	export class ProxyCoreDownloadRequest {
	    core: string;
	    goos: string;
	    goarch: string;
	    proxyConfig: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCoreDownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.proxyConfig = source["proxyConfig"];
	        this.version = source["version"];
	    }
	}
	export class ProxyCoreStatusResult {
	    core: string;
	    goos: string;
	    goarch: string;
	    installed: boolean;
	    configured: boolean;
	    active: boolean;
	    binaryPath: string;
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCoreStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.installed = source["installed"];
	        this.configured = source["configured"];
	        this.active = source["active"];
	        this.binaryPath = source["binaryPath"];
	        this.source = source["source"];
	        this.message = source["message"];
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
	export class ProxyLocationOption {
	    label: string;
	    timezone: string;
	    lang: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyLocationOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.timezone = source["timezone"];
	        this.lang = source["lang"];
	    }
	}
	export class ProxyLocationResolveResult {
	    proxyId: string;
	    ok: boolean;
	    auto: boolean;
	    source: string;
	    error: string;
	    ip: string;
	    country: string;
	    region: string;
	    city: string;
	    timezone: string;
	    lang: string;
	    health?: ProxyIPHealthResult;
	    alternates?: ProxyLocationOption[];
	    resolvedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyLocationResolveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.auto = source["auto"];
	        this.source = source["source"];
	        this.error = source["error"];
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.region = source["region"];
	        this.city = source["city"];
	        this.timezone = source["timezone"];
	        this.lang = source["lang"];
	        this.health = this.convertValues(source["health"], ProxyIPHealthResult);
	        this.alternates = this.convertValues(source["alternates"], ProxyLocationOption);
	        this.resolvedAt = source["resolvedAt"];
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
	export class ProxyTestResult {
	    proxyId: string;
	    ok: boolean;
	    latencyMs: number;
	    engine: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.latencyMs = source["latencyMs"];
	        this.engine = source["engine"];
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
	    coreBackend: string;
	
	    static createFrom(source: any = {}) {
	        return new CoreExtendedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.chromeVersion = source["chromeVersion"];
	        this.instanceCount = source["instanceCount"];
	        this.coreBackend = source["coreBackend"];
	    }
	}
	export class CoreInput {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	    coreBackend: string;
	    coreEnv: string[];
	
	    static createFrom(source: any = {}) {
	        return new CoreInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	        this.coreBackend = source["coreBackend"];
	        this.coreEnv = source["coreEnv"];
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
	export class Extension {
	    extensionId: string;
	    name: string;
	    version: string;
	    description: string;
	    iconDataUrl: string;
	    manifestJson: string;
	    sourceUrl: string;
	    installDir: string;
	    enabled: boolean;
	    installedAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.iconDataUrl = source["iconDataUrl"];
	        this.manifestJson = source["manifestJson"];
	        this.sourceUrl = source["sourceUrl"];
	        this.installDir = source["installDir"];
	        this.enabled = source["enabled"];
	        this.installedAt = source["installedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ExtensionLookupResult {
	    extensionId: string;
	    name: string;
	    version: string;
	    description: string;
	    storeUrl: string;
	    installable: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionLookupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.storeUrl = source["storeUrl"];
	        this.installable = source["installable"];
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
	    restoreLastSession: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    proxyBindSourceId: string;
	    proxyBindSourceUrl: string;
	    proxyBindName: string;
	    proxyBindUpdatedAt: string;
	    memoryLimitMb: number;
	    launchArgs: string[];
	    lastLaunchArgs: string[];
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
	    deletedAt: string;
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
	        this.restoreLastSession = source["restoreLastSession"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.proxyBindSourceId = source["proxyBindSourceId"];
	        this.proxyBindSourceUrl = source["proxyBindSourceUrl"];
	        this.proxyBindName = source["proxyBindName"];
	        this.proxyBindUpdatedAt = source["proxyBindUpdatedAt"];
	        this.memoryLimitMb = source["memoryLimitMb"];
	        this.launchArgs = source["launchArgs"];
	        this.lastLaunchArgs = source["lastLaunchArgs"];
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
	        this.deletedAt = source["deletedAt"];
	        this.lastStartAt = source["lastStartAt"];
	        this.lastStopAt = source["lastStopAt"];
	    }
	}
	export class ProfileCopyOptions {
	    mode: string;
	    automationTargets: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProfileCopyOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.automationTargets = source["automationTargets"];
	    }
	}
	export class ProfileExtensionSettings {
	    profileId: string;
	    configured: boolean;
	    extensionIds: string[];
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileExtensionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.configured = source["configured"];
	        this.extensionIds = source["extensionIds"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProfileInput {
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    restoreLastSession: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    memoryLimitMb: number;
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
	        this.restoreLastSession = source["restoreLastSession"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.memoryLimitMb = source["memoryLimitMb"];
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
	    defaultStartUrls: string[];
	    lightStartEnabled: boolean;
	    restoreLastSession: boolean;
	    startReadyTimeoutMs: number;
	    startStableWindowMs: number;
	    defaultConnectorType: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userDataRoot = source["userDataRoot"];
	        this.defaultFingerprintArgs = source["defaultFingerprintArgs"];
	        this.defaultLaunchArgs = source["defaultLaunchArgs"];
	        this.defaultStartUrls = source["defaultStartUrls"];
	        this.lightStartEnabled = source["lightStartEnabled"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.startReadyTimeoutMs = source["startReadyTimeoutMs"];
	        this.startStableWindowMs = source["startStableWindowMs"];
	        this.defaultConnectorType = source["defaultConnectorType"];
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
	    openOnStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.openOnStart = source["openOnStart"];
	    }
	}
	export class BrowserCore {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	    coreBackend?: string;
	    coreEnv?: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserCore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	        this.coreBackend = source["coreBackend"];
	        this.coreEnv = source["coreEnv"];
	    }
	}
	export class BrowserProxy {
	    proxyId: string;
	    proxyName: string;
	    proxyConfig: string;
	    preferredKernel?: string;
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
	        this.preferredKernel = source["preferredKernel"];
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
	export class ProxyCheckTarget {
	    id: string;
	    name: string;
	    type: string;
	    url: string;
	    parser?: string;
	    timeoutMs?: number;
	    expectedStatus?: number[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyCheckTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.parser = source["parser"];
	        this.timeoutMs = source["timeoutMs"];
	        this.expectedStatus = source["expectedStatus"];
	    }
	}
	export class ProxyCheckConfig {
	    bridgeStartTimeoutMs: number;
	    speedTargetId: string;
	    ipHealthTargetId: string;
	    targets: ProxyCheckTarget[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyCheckConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bridgeStartTimeoutMs = source["bridgeStartTimeoutMs"];
	        this.speedTargetId = source["speedTargetId"];
	        this.ipHealthTargetId = source["ipHealthTargetId"];
	        this.targets = this.convertValues(source["targets"], ProxyCheckTarget);
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

export namespace launchcode {
	
	export class LaunchRequestParams {
	    launchArgs: string[];
	    startUrls: string[];
	    skipDefaultStartUrls: boolean;
	    proxyId: string;
	    proxyConfig: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchRequestParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.launchArgs = source["launchArgs"];
	        this.startUrls = source["startUrls"];
	        this.skipDefaultStartUrls = source["skipDefaultStartUrls"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
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

export namespace proxy {
	
	export class DnsDiagnosticSummary {
	    hasConfig: boolean;
	    sourceFormat: string;
	    enhancedMode: string;
	    nameserverCount: number;
	    fallbackCount: number;
	    xrayServerCount: number;
	    unsupported: string[];
	
	    static createFrom(source: any = {}) {
	        return new DnsDiagnosticSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasConfig = source["hasConfig"];
	        this.sourceFormat = source["sourceFormat"];
	        this.enhancedMode = source["enhancedMode"];
	        this.nameserverCount = source["nameserverCount"];
	        this.fallbackCount = source["fallbackCount"];
	        this.xrayServerCount = source["xrayServerCount"];
	        this.unsupported = source["unsupported"];
	    }
	}
	export class ProxyRuntimeDiagnostic {
	    workDir: string;
	    configPath: string;
	    stderrPath: string;
	    logPath: string;
	    errorPath: string;
	    recentLogs: Record<string, string>;
	    bridgeAlive: boolean;
	    bridgePort: number;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyRuntimeDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workDir = source["workDir"];
	        this.configPath = source["configPath"];
	        this.stderrPath = source["stderrPath"];
	        this.logPath = source["logPath"];
	        this.errorPath = source["errorPath"];
	        this.recentLogs = source["recentLogs"];
	        this.bridgeAlive = source["bridgeAlive"];
	        this.bridgePort = source["bridgePort"];
	        this.lastError = source["lastError"];
	    }
	}
	export class ProxyBuildDiagnostic {
	    proxyId: string;
	    proxyName: string;
	    found: boolean;
	    ok: boolean;
	    engine: string;
	    nodeKey: string;
	    rawConfigMasked: string;
	    dnsServers: string;
	    dnsSummary: DnsDiagnosticSummary;
	    standardProxy: string;
	    outbounds: any[];
	    routes: any[];
	    inbound: Record<string, any>;
	    outbound: Record<string, any>;
	    runtime: ProxyRuntimeDiagnostic;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyBuildDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.proxyName = source["proxyName"];
	        this.found = source["found"];
	        this.ok = source["ok"];
	        this.engine = source["engine"];
	        this.nodeKey = source["nodeKey"];
	        this.rawConfigMasked = source["rawConfigMasked"];
	        this.dnsServers = source["dnsServers"];
	        this.dnsSummary = this.convertValues(source["dnsSummary"], DnsDiagnosticSummary);
	        this.standardProxy = source["standardProxy"];
	        this.outbounds = source["outbounds"];
	        this.routes = source["routes"];
	        this.inbound = source["inbound"];
	        this.outbound = source["outbound"];
	        this.runtime = this.convertValues(source["runtime"], ProxyRuntimeDiagnostic);
	        this.errors = source["errors"];
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

