// 统一的轻量表单校验工具，替换各表单 ad-hoc 的 `if(!x.trim()) toast.error(...)`。

export interface RequiredField {
  value: string | undefined | null
  label: string
}

// requireFields 返回第一个空字段的错误信息；全部通过返回 null。
export function requireFields(fields: RequiredField[]): string | null {
  for (const f of fields) {
    if (!f.value || !f.value.trim()) {
      return `请输入${f.label}`
    }
  }
  return null
}
