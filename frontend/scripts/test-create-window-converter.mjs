import assert from 'node:assert/strict'
import { createServer } from 'vite'

const server = await createServer({
  root: process.cwd(),
  logLevel: 'error',
  server: { middlewareMode: true, hmr: false },
})

try {
  const {
    createWindowFormToProfileInput,
    restoreCreateWindowFormState,
  } = await server.ssrLoadModule('/src/modules/browser/utils/createWindowConverter.ts')

  const core144 = { coreId: 'core-144', coreName: 'Chrome fingerprint-chromium-144', corePath: 'chrome/fingerprint-chromium-144', isDefault: true }
  const expectedWindowsChrome144UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.7559.132 Safari/537.36'
  const legacyWindowsChrome149UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36'

  const formState = {
    profileName: 'Stage 1 profile',
    userDataDir: 'C:/tmp/profile-stage-1',
    coreId: '',
    fingerprintArgs: ['--user-agent=ManualUA'],
    proxyId: '',
    proxyConfig: '',
    launchArgs: ['--accept-language=fr-FR'],
    tags: ['stage1'],
    keywords: ['converter'],
    browserVersion: 'RoxyChrome 149',
    system: 'windows',
    userAgent: 'GeneratedUA',
    language: 'en-US',
    uiLanguage: 'en-US',
    timezone: 'Asia/Shanghai',
    urls: 'example.com https://openai.com/docs',
    windowWidth: '1280',
    windowHeight: '720',
    resolution: 'custom',
    audio: false,
    image: false,
    video: false,
    searchEngine: 'bing',
    hardwareAcceleration: 'off',
    sandbox: 'on',
    webgpu: 'disabled',
    hardwareConcurrency: '8',
    deviceMemory: '16',
    doNotTrack: 'off',
    webrtc: 'replace',
    geolocationDisplay: 'allow',
    fontFingerprint: 'system',
    webgl: 'real',
    webglInfo: 'real',
    webglVendor: 'Ignored Vendor',
    webglRenderer: 'Ignored Renderer',
    canvas: 'real',
    audioContext: 'real',
    clientRects: 'real',
    cookies: '[{"name":"secret"}]',
    randomFingerprintOnStart: true,
    googleLogin: 'off',
  }

  const payload = createWindowFormToProfileInput(formState, {
    launchArgsText: '--window-size=1024,768',
    cores: [{ coreId: 'core-149', coreName: 'RoxyChrome 149', corePath: '', isDefault: true }],
    selectedExtensionIds: ['ext-a'],
  })

  assert.equal(payload.coreId, 'core-149')
  assert.equal(payload.profileName, formState.profileName)
  assert.deepEqual(payload.tags, ['stage1'])
  assert.ok(payload.fingerprintArgs.includes('--user-agent=ManualUA'))
  assert.ok(!payload.fingerprintArgs.includes('--user-agent=GeneratedUA'))
  assert.ok(payload.fingerprintArgs.includes('--fingerprint-platform=windows'))
  assert.ok(payload.fingerprintArgs.includes('--lang=en-US'))
  assert.ok(payload.fingerprintArgs.includes('--timezone=Asia/Shanghai'))
  assert.ok(payload.fingerprintArgs.includes('--ant-timezone-mode=real'))
  assert.ok(payload.fingerprintArgs.includes('--fingerprint-hardware-concurrency=8'))
  assert.ok(payload.fingerprintArgs.includes('--fingerprint-device-memory=16'))
  assert.ok(payload.fingerprintArgs.includes('--fingerprint-do-not-track=false'))
  assert.ok(payload.fingerprintArgs.includes('--webrtc-ip-handling-policy=default_public_interface_only'))
  assert.ok(payload.fingerprintArgs.includes('--ant-geolocation-permission=allow'))
  assert.ok(payload.fingerprintArgs.includes('--disable-spoofing=font,gpu,canvas,audio,clientrects'))
  // Chrome 144+ 已废弃自定义 WebGL 参数，转换器不应再生成它们。
  assert.ok(!payload.fingerprintArgs.some(arg => arg.startsWith('--fingerprint-webgl-vendor=')))
  assert.ok(!payload.fingerprintArgs.some(arg => arg.startsWith('--fingerprint-webgl-renderer=')))

  assert.ok(payload.launchArgs.includes('--window-size=1024,768'))
  assert.ok(!payload.launchArgs.includes('--window-size=1280,720'))
  assert.ok(payload.launchArgs.includes('--accept-language=fr-FR'))
  assert.ok(!payload.launchArgs.includes('--accept-language=en-US'))
  assert.ok(payload.launchArgs.includes('--mute-audio'))
  assert.ok(payload.launchArgs.includes('--blink-settings=imagesEnabled=false'))
  assert.ok(payload.launchArgs.includes('--autoplay-policy=user-gesture-required'))
  assert.ok(payload.launchArgs.includes('--ant-search-engine=bing'))
  assert.ok(payload.launchArgs.includes('--disable-gpu'))
  assert.ok(payload.launchArgs.includes('--no-sandbox'))
  assert.ok(payload.launchArgs.includes('--disable-features=WebGPU'))
  assert.ok(payload.launchArgs.includes('--allow-browser-signin=false'))
  assert.ok(payload.launchArgs.includes('https://example.com/'))
  assert.ok(payload.launchArgs.includes('https://openai.com/docs'))

  const config = JSON.parse(payload.profileConfig)
  assert.equal(config.version, 1)
  assert.equal(config.formState.cookies, '')
  assert.equal(config.formState.webglVendor, undefined)
  assert.equal(config.formState.webglRenderer, undefined)
  assert.equal(config.postCreateActions.importCookies, '[{"name":"secret"}]')
  assert.equal(config.formState.randomFingerprintOnStart, true)
  assert.deepEqual(config.selectedExtensionIds, ['ext-a'])

  const versionAlignedPayload = createWindowFormToProfileInput({
    profileName: 'Version aligned',
    userDataDir: '',
    coreId: 'core-144',
    fingerprintArgs: [],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [],
    tags: [],
    keywords: [],
    system: 'windows',
  }, {
    cores: [core144],
    coreVersions: { 'core-144': '144.0.7559.132' },
  })
  assert.ok(versionAlignedPayload.fingerprintArgs.includes(`--user-agent=${expectedWindowsChrome144UA}`))

  const migratedLegacyUaPayload = createWindowFormToProfileInput({
    profileName: 'Migrated legacy UA',
    userDataDir: '',
    coreId: 'core-144',
    fingerprintArgs: [],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [],
    tags: [],
    keywords: [],
    system: 'windows',
    userAgent: legacyWindowsChrome149UA,
  }, {
    cores: [core144],
    coreVersions: { 'core-144': '144.0.7559.132' },
  })
  assert.ok(migratedLegacyUaPayload.fingerprintArgs.includes(`--user-agent=${expectedWindowsChrome144UA}`))
  assert.ok(!migratedLegacyUaPayload.fingerprintArgs.some(arg => arg.includes('Chrome/149.0.0.0')))

  const restoredFromConfig = restoreCreateWindowFormState({
    profileId: 'profile-1',
    ...payload,
    profileConfig: payload.profileConfig,
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: '',
    updatedAt: '',
  })
  assert.equal(restoredFromConfig.userAgent, 'GeneratedUA')
  assert.equal(restoredFromConfig.cookies, '')
  assert.equal(restoredFromConfig.windowWidth, '1280')

  const restoredFromArgs = restoreCreateWindowFormState({
    profileId: 'profile-2',
    profileName: 'Legacy profile',
    userDataDir: 'C:/tmp/legacy',
    coreId: 'legacy-core',
    fingerprintArgs: [
      '--user-agent=LegacyUA',
      '--fingerprint-platform=linux',
      '--lang=ja-JP',
      '--ant-timezone-mode=auto',
      '--fingerprint-do-not-track=true',
      '--webrtc-ip-handling-policy=disable_non_proxied_udp',
      '--ant-geolocation-permission=block',
      '--fingerprint-webgl-vendor=Legacy Vendor',
      '--fingerprint-webgl-renderer=Legacy Renderer',
      '--disable-spoofing=font,gpu,canvas,audio,clientrects',
    ],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [
      '--accept-language=ja-JP',
      '--start-maximized',
      '--mute-audio',
      '--blink-settings=imagesEnabled=false',
      '--autoplay-policy=user-gesture-required',
      '--ant-search-engine=duckduckgo',
      '--disable-gpu',
      '--no-sandbox',
      '--disable-features=Foo,WebGPU',
      '--allow-browser-signin=false',
      'https://legacy.example/',
    ],
    tags: [],
    keywords: [],
    groupId: '',
    profileConfig: '',
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: '',
    updatedAt: '',
  })
  assert.equal(restoredFromArgs.userAgent, 'LegacyUA')
  assert.equal(restoredFromArgs.system, 'linux')
  assert.equal(restoredFromArgs.language, 'ja-JP')
  assert.equal(restoredFromArgs.timezone, 'auto')
  assert.equal(restoredFromArgs.uiLanguage, 'ja-JP')
  assert.equal(restoredFromArgs.resolution, 'fullscreen')
  assert.equal(restoredFromArgs.audio, false)
  assert.equal(restoredFromArgs.image, false)
  assert.equal(restoredFromArgs.video, false)
  assert.equal(restoredFromArgs.searchEngine, 'duckduckgo')
  assert.equal(restoredFromArgs.hardwareAcceleration, 'off')
  assert.equal(restoredFromArgs.sandbox, 'on')
  assert.equal(restoredFromArgs.webgpu, 'disabled')
  assert.equal(restoredFromArgs.doNotTrack, 'on')
  assert.equal(restoredFromArgs.webrtc, 'disabled')
  assert.equal(restoredFromArgs.geolocationDisplay, 'deny')
  assert.equal(restoredFromArgs.webgl, 'real')
  assert.equal(restoredFromArgs.webglInfo, 'real')
  assert.equal(restoredFromArgs.canvas, 'real')
  assert.equal(restoredFromArgs.audioContext, 'real')
  assert.equal(restoredFromArgs.clientRects, 'real')
  assert.equal(restoredFromArgs.fontFingerprint, 'system')
  assert.equal(restoredFromArgs.webglVendor, 'Legacy Vendor')
  assert.equal(restoredFromArgs.webglRenderer, 'Legacy Renderer')
  assert.equal(restoredFromArgs.urls, 'https://legacy.example/')
  assert.equal(restoredFromArgs.googleLogin, 'off')

  // 编辑回环：还原一个已落库的窗口 → 改受管控件 → 重新转换。
  // 验证控件新值覆盖旧受管参数，且非受管手填参数（--fingerprint= 种子、自定义开关）保留。
  const editProfile = {
    profileId: 'profile-edit',
    profileName: 'Edit roundtrip',
    userDataDir: 'C:/tmp/edit',
    coreId: 'core-149',
    fingerprintArgs: [
      '--fingerprint=12345',            // 非受管：必须保留
      '--user-agent=OldUA',
      '--fingerprint-platform=windows',
      '--timezone=Asia/Shanghai',
      '--ant-timezone-mode=real',
    ],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [
      '--window-size=800,600',
      '--ant-search-engine=google',
      '--custom-manual-flag',           // 非受管：必须保留
    ],
    tags: [],
    keywords: [],
    groupId: '',
    profileConfig: '',
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: '',
    updatedAt: '',
  }
  const editForm = restoreCreateWindowFormState(editProfile)
  // 还原后受管参数已被剥离，只剩非受管手填项。
  assert.deepEqual(editForm.fingerprintArgs, ['--fingerprint=12345'])
  assert.deepEqual(editForm.launchArgs, ['--custom-manual-flag'])
  assert.equal(editForm.timezone, 'Asia/Shanghai')
  assert.equal(editForm.windowWidth, '800')
  // 用户在富表单里改了时区、窗口大小、搜索引擎。
  const editedForm = {
    ...editForm,
    timezone: 'America/New_York',
    windowWidth: '1440',
    windowHeight: '900',
    searchEngine: 'bing',
  }
  const editedPayload = createWindowFormToProfileInput(editedForm, {
    launchArgsText: editForm.launchArgs.join('\n'),
    cores: [{ coreId: 'core-149', coreName: 'RoxyChrome 149', corePath: '', isDefault: true }],
    selectedExtensionIds: [],
  })
  // 受管参数取控件新值，旧值不残留。
  assert.ok(editedPayload.fingerprintArgs.includes('--timezone=America/New_York'))
  assert.ok(!editedPayload.fingerprintArgs.includes('--timezone=Asia/Shanghai'))
  assert.ok(editedPayload.launchArgs.includes('--window-size=1440,900'))
  assert.ok(!editedPayload.launchArgs.includes('--window-size=800,600'))
  assert.ok(editedPayload.launchArgs.includes('--ant-search-engine=bing'))
  assert.ok(!editedPayload.launchArgs.includes('--ant-search-engine=google'))
  // 非受管手填参数保留。
  assert.ok(editedPayload.fingerprintArgs.includes('--fingerprint=12345'))
  assert.ok(editedPayload.launchArgs.includes('--custom-manual-flag'))

  // 窗口位置：九宫格 + 屏幕尺寸换算为 --window-position=x,y。
  const positionForm = {
    profileName: 'Position',
    userDataDir: '',
    coreId: '',
    fingerprintArgs: [],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [],
    tags: [],
    keywords: [],
    resolution: 'custom',
    windowWidth: '1000',
    windowHeight: '800',
    windowPosition: 'bottom-right',
  }
  const positionPayload = createWindowFormToProfileInput(positionForm, {
    screen: { width: 1920, height: 1080 },
  })
  // bottom-right: x=(1920-1000)=920, y=(1080-800)=280
  assert.ok(positionPayload.launchArgs.includes('--window-position=920,280'))

  // center: x=(1920-1000)/2=460, y=(1080-800)/2=140
  const centerPayload = createWindowFormToProfileInput(
    { ...positionForm, windowPosition: 'center' },
    { screen: { width: 1920, height: 1080 } },
  )
  assert.ok(centerPayload.launchArgs.includes('--window-position=460,140'))

  // top-left 等于系统默认放置：不生成窗口位置参数。
  const topLeftPayload = createWindowFormToProfileInput(
    { ...positionForm, windowPosition: 'top-left' },
    { screen: { width: 1920, height: 1080 } },
  )
  assert.ok(!topLeftPayload.launchArgs.some(arg => arg.startsWith('--window-position=')))

  // 缺少屏幕尺寸时不生成（无法可靠定位）。
  const noScreenPayload = createWindowFormToProfileInput(
    { ...positionForm, windowPosition: 'bottom-right' },
    {},
  )
  assert.ok(!noScreenPayload.launchArgs.some(arg => arg.startsWith('--window-position=')))

  // 全屏时跳过窗口位置（已 --start-maximized）。
  const fullscreenPayload = createWindowFormToProfileInput(
    { ...positionForm, resolution: 'fullscreen', windowPosition: 'bottom-right' },
    { screen: { width: 1920, height: 1080 } },
  )
  assert.ok(fullscreenPayload.launchArgs.includes('--start-maximized'))
  assert.ok(!fullscreenPayload.launchArgs.some(arg => arg.startsWith('--window-position=')))

  // 窗口位置参数视为受管：编辑回环时从存量 args 剥离，避免旧坐标残留。
  const stalePositionForm = restoreCreateWindowFormState({
    ...editProfile,
    launchArgs: ['--window-position=10,20', '--custom-manual-flag'],
  })
  assert.ok(!stalePositionForm.launchArgs.some(arg => arg.startsWith('--window-position=')))
  assert.ok(stalePositionForm.launchArgs.includes('--custom-manual-flag'))

  // 地理位置：自定义经纬度生成 --ant-geolocation=lat,lon（进 fingerprintArgs，由后端抽取）。
  const geoForm = {
    profileName: 'Geo',
    userDataDir: '',
    coreId: '',
    fingerprintArgs: [],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [],
    tags: [],
    keywords: [],
    geolocation: 'custom',
    latitude: '48.8566',
    longitude: '2.3522',
  }
  const geoPayload = createWindowFormToProfileInput(geoForm, {})
  assert.ok(geoPayload.fingerprintArgs.includes('--ant-geolocation=48.8566,2.3522'))

  // auto（基于 IP 匹配）：不生成显式经纬度，由后端按代理出口 IP 推导。
  const autoGeoPayload = createWindowFormToProfileInput({ ...geoForm, geolocation: 'auto' }, {})
  assert.ok(!autoGeoPayload.fingerprintArgs.some(arg => arg.startsWith('--ant-geolocation=')))

  // 经纬度超范围：不生成参数（避免给内核非法值）。
  const badGeoPayload = createWindowFormToProfileInput({ ...geoForm, latitude: '999', longitude: '2.3522' }, {})
  assert.ok(!badGeoPayload.fingerprintArgs.some(arg => arg.startsWith('--ant-geolocation=')))

  // 旧 profile 回显：从 --ant-geolocation= 反解析经纬度，并切到 custom。
  const restoredGeo = restoreCreateWindowFormState({
    ...editProfile,
    fingerprintArgs: ['--ant-geolocation=35.6762,139.6503,100', '--fingerprint=12345'],
    launchArgs: [],
  })
  assert.equal(restoredGeo.geolocation, 'custom')
  assert.equal(restoredGeo.latitude, '35.6762')
  assert.equal(restoredGeo.longitude, '139.6503')

  console.log('createWindowConverter tests passed')
} finally {
  await server.close()
}
