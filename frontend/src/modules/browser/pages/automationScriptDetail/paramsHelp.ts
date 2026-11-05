import type { AutomationScriptRecord } from "../../automationScripts";

const PROTON_MAIL_FIRST_MESSAGE_SCRIPT_ID = "proton-mail-first-message";

export interface ScriptParamFieldDoc {
  name: string;
  description: string;
  defaultValue?: string;
  note?: string;
}

export interface ScriptParamsHelpContent {
  title: string;
  summary: string;
  docs: ScriptParamFieldDoc[];
}

const PROTON_MAIL_PARAMS_HELP: ScriptParamsHelpContent = {
  title: "邮件筛选字段说明",
  summary: "优先只填 searchQuery、senderEmail；recipient 保持可选。",
  docs: [
    {
      name: "searchQuery",
      description:
        "唯一主搜索条件，只把这个值填进 Proton 搜索框。多个候选词可用逗号分隔；脚本会和 senderEmail、recipient 做组合搜索。",
      defaultValue: "OpenAI, ChatGPT",
      note: "searchQuery 和 senderEmail 默认各只取前 2 个候选，所以组合搜索最多 4 次。",
    },
    {
      name: "recipient",
      description: "接收者邮箱关键词，用来约束结果，不直接写进搜索框；每次都会和当前 searchQuery / senderEmail 一起提交。",
      defaultValue: "",
      note: "兼容旧字段 recipientQuery；同邮箱里命中过多时再补。",
    },
    {
      name: "senderEmail",
      description:
        "发件邮箱，用来约束结果，不直接写进搜索框。支持逗号分隔多个候选值；脚本会和 searchQuery、recipient 做组合搜索。",
      defaultValue: "otp@tm1.openai.com, noreply@tm.openai.com",
    },
    {
      name: "inboxUrl",
      description: "可选。只有多账号或特殊路由时才需要传。",
      defaultValue: "https://mail.proton.me/u/0/inbox",
    },
    {
      name: "timeoutMs",
      description: "可选。整次脚本执行超时，默认值通常够用。",
      defaultValue: "90000",
    },
    {
      name: "maxBodyChars",
      description: "可选。正文很长时用它限制返回体积。",
      defaultValue: "12000",
    },
  ],
};


export function getScriptParamsHelp(
  script: AutomationScriptRecord,
): ScriptParamsHelpContent | null {
  if (script.id === PROTON_MAIL_FIRST_MESSAGE_SCRIPT_ID) {
    return PROTON_MAIL_PARAMS_HELP;
  }
  return null;
}
