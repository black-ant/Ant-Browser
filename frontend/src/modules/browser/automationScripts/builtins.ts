import {
  AUTOMATION_SCRIPT_MANIFEST_VERSION,
  AUTOMATION_SCRIPT_PACKAGE_FORMAT,
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  type AutomationScriptRecord,
} from "./definitions";
import { createAutomationScriptPublicAPIConfig } from "./publicApi";
import {
  normalizeAutomationScriptTargetConfig,
} from "./targets";

const BACKEND_BUILTIN_SCRIPT_PLACEHOLDER = `module.exports.run = async () => {
  throw new Error('内置脚本源码由后端 demo-library 提供，请在桌面应用后端环境中加载或从脚本包导入。')
}`;


function nowIso(): string {
  return new Date().toISOString();
}

export {
  buildParamsTemplate,
  buildScriptTemplate,
  buildSelectorTemplate,
  buildNotesTemplate,
  normalizeDualInstanceRuntimeParamsText,
} from "./builtinsTemplates";
import {
  buildDualInstanceRuntimeParamsText,
  buildDualInstanceRuntimeScriptText,
} from "./builtinsTemplates";

export function createNewsTxtScriptDraft(): AutomationScriptRecord {
  const createdAt = nowIso();

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: "news-query-txt",
    name: "查询新闻并写 TXT",
    description: "通过 Bing 搜索新闻关键词，提取结果并写入本地 txt 文件。",
    type: "playwright-cdp",
    status: "ready",
    entryFile: "index.cjs",
    tags: ["Playwright", "新闻", "TXT"],
    selectorText: "",
    paramsText: `{
  "keyword": "OpenAI",
  "limit": 10,
  "timeRange": "week",
  "outputFileName": "openai-news.txt",
  "timeoutMs": 30000,
  "waitAfterLoadMs": 1500,
  "captureScreenshot": false
}`,
    scriptText: BACKEND_BUILTIN_SCRIPT_PLACEHOLDER,
    notes:
      "脚本会优先使用 Bing 搜索真实新闻结果，并自动追加时间过滤、排除问答/聚合站点、回退查询词和质量校验；只有达到新闻质量门槛时才会判定成功，并把结果写入本地 txt。执行成功后可在结果里的 outputPath 找到文件。",
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: createAutomationScriptPublicAPIConfig(),
    source: {
      type: "builtin",
      uri: "repo://backend/internal/automation/demo-library/news-query-txt",
      ref: "HEAD",
      path: "news-query-txt",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function createDualInstanceRuntimeScriptDraft(): AutomationScriptRecord {
  const createdAt = nowIso();

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
    name: "双实例启动与 Runtime 切换",
    description:
      "通过 Launch API 分别启动两个实例，切换 Runtime 会话后交给 OpenClaw 执行。",
    type: "launch-api",
    status: "ready",
    entryFile: "index.cjs",
    tags: ["Launch API", "OpenClaw", "双实例"],
    selectorText: "",
    paramsText: buildDualInstanceRuntimeParamsText(),
    scriptText: buildDualInstanceRuntimeScriptText(),
    notes:
      "先通过接口启动两个实例并切换 Runtime 会话；随后把实例信息交给 OpenClaw 执行自动化动作。",
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: createAutomationScriptPublicAPIConfig(),
    source: {
      type: "builtin",
      uri: "repo://backend/internal/automation/demo-library/dual-instance-runtime-switch",
      ref: "HEAD",
      path: "dual-instance-runtime-switch",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function createWebImageGenerateDownloadScriptDraft(): AutomationScriptRecord {
  const createdAt = nowIso();

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: "web-image-generate-download",
    name: "网页图片生成并下载",
    description:
      "打开 ChatGPT，发送图片生成消息，等待图片生成后下载图片。",
    type: "playwright-cdp",
    status: "draft",
    entryFile: "index.cjs",
    tags: ["Playwright", "图片生成", "下载"],
    selectorText: "",
    paramsText: `{
  "pageUrl": "https://chatgpt.com/",
  "prompt": "A cinematic chrome ant browser mascot, premium product lighting",
  "outputFileName": "generated-image.png",
  "timeoutMs": 300000,
  "waitAfterLoadMs": 1200,
  "settleMs": 2500,
  "captureScreenshot": false
}`,
    scriptText: BACKEND_BUILTIN_SCRIPT_PLACEHOLDER,
    notes:
      "脚本默认打开 ChatGPT，输入图片生成提示词并发送；每次运行会排除运行前已经存在的图片，避免 gateway 请求 n>1 时重复返回旧图。页面选择器、下载文件名等可以通过 params 覆盖。",
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: {
      ...createAutomationScriptPublicAPIConfig(),
      enabled: true,
      path: "image/chatgpt-generate-download",
      timeoutMs: 300000,
      requestBodyText: `{
  "instance": {
    "type": "existing",
    "selector": {
      "code": "BUYER_001"
    }
  },
  "params": {
    "prompt": "{{prompt}}"
  },
  "timeoutMs": 300000
}`,
      responseBodyText: `{
  "ok": true,
  "status": "completed",
  "summary": "图片已生成并下载。",
  "outputPath": "\${artifactsDir}/generated-image.png",
  "downloadAddress": "\${artifactsDir}/generated-image.png"
}`,
      variables: [
        {
          name: "prompt",
          defaultValue:
            "A cinematic chrome ant browser mascot, premium product lighting",
          description: "发送到 ChatGPT 的图片生成提示词。",
          required: true,
        },
      ],
    },
    source: {
      type: "builtin",
      uri: "repo://backend/internal/automation/demo-library/web-image-generate-download",
      ref: "HEAD",
      path: "web-image-generate-download",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function buildDefaultAutomationScripts(): AutomationScriptRecord[] {
  return [
    createNewsTxtScriptDraft(),
    createDualInstanceRuntimeScriptDraft(),
    createWebImageGenerateDownloadScriptDraft(),
  ];
}
