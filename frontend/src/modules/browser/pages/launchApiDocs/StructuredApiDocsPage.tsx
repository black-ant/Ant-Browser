import type { ReactNode } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Button } from '../../../../shared/components'
import { LaunchDocsCodeBlock } from './LaunchDocsCodeBlock'
import {
  getStructuredApiSectionEndpoints,
  isStructuredApiEndpointDocId,
  STRUCTURED_API_ENDPOINT_DOC_MAP,
  STRUCTURED_API_SECTION_DOC_MAP,
  type StructuredApiDocId,
  type StructuredApiField,
  type StructuredApiMethod,
  type StructuredApiResponseCode,
  type StructuredApiSectionId,
} from './structuredApiDocs'

interface StructuredApiDocsPageProps {
  docId: StructuredApiDocId
  launchBaseUrl: string
  authHeader: string
  onOpenDoc: (id: StructuredApiDocId) => void
}

function MethodBadge({ method }: { method: StructuredApiMethod }) {
  const className = {
    GET: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    POST: 'border-sky-200 bg-sky-50 text-sky-700',
    PUT: 'border-amber-200 bg-amber-50 text-amber-700',
    DELETE: 'border-rose-200 bg-rose-50 text-rose-700',
    WS: 'border-violet-200 bg-violet-50 text-violet-700',
  }[method]

  return (
    <span className={`inline-flex h-8 items-center rounded-full border px-3 text-xs font-semibold tracking-[0.16em] ${className}`}>
      {method}
    </span>
  )
}

function FieldTable({ fields }: { fields: StructuredApiField[] }) {
  if (!fields.length) {
    return (
      <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-5 py-4 text-sm text-[var(--color-text-muted)]">
        无参数
      </div>
    )
  }

  return (
    <div className="overflow-x-auto rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-sm)]">
      <table className="w-full min-w-[680px] text-sm">
        <thead className="bg-[var(--color-bg-muted)] text-left">
          <tr>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">字段</th>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">位置</th>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">类型</th>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">必填</th>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">说明</th>
          </tr>
        </thead>
        <tbody>
          {fields.map((field) => (
            <tr key={`${field.location}-${field.name}`} className="border-t border-[var(--color-border-muted)]">
              <td className="px-4 py-2.5 font-mono text-[var(--color-text-primary)]">{field.name}</td>
              <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">{field.location}</td>
              <td className="px-4 py-2.5 font-mono text-[var(--color-text-secondary)]">{field.type}</td>
              <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">{field.required ? '是' : '否'}</td>
              <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">{field.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ResponseCodeTable({ items }: { items: StructuredApiResponseCode[] }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-sm)]">
      <table className="w-full min-w-[420px] text-sm">
        <thead className="bg-[var(--color-bg-muted)] text-left">
          <tr>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">状态码</th>
            <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">说明</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.code} className="border-t border-[var(--color-border-muted)]">
              <td className="px-4 py-2.5 font-mono text-[var(--color-text-primary)]">{item.code}</td>
              <td className="px-4 py-2.5 text-[var(--color-text-secondary)]">{item.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function SectionTitle({ title, description }: { title: string, description?: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">{title}</h2>
      {description ? (
        <p className="max-w-3xl text-sm leading-6 text-[var(--color-text-secondary)]">{description}</p>
      ) : null}
    </div>
  )
}

function StructuredApiSectionPage({
  docId,
  onOpenDoc,
}: {
  docId: StructuredApiSectionId
  onOpenDoc: (id: StructuredApiDocId) => void
}) {
  const section = STRUCTURED_API_SECTION_DOC_MAP[docId]
  const endpoints = getStructuredApiSectionEndpoints(docId)

  return (
    <div className="space-y-6">
      <section className="rounded-[28px] border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-6 shadow-[var(--shadow-md)]">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight text-[var(--color-text-primary)]">
            {section.title}
          </h1>
        </div>
      </section>

      <section className="space-y-4">
        <SectionTitle title="功能介绍" />
        <div className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-5 shadow-[var(--shadow-sm)]">
          <ul className="space-y-3 text-sm leading-7 text-[var(--color-text-secondary)]">
            {section.highlights.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </div>
      </section>

      <section className="space-y-4">
        <SectionTitle title="接口" />
        <div className="overflow-x-auto rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-sm)]">
          <table className="w-full min-w-[860px] text-sm">
            <thead className="bg-[var(--color-bg-muted)] text-left">
              <tr>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">方法</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">路径</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">用途</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">详情</th>
              </tr>
            </thead>
            <tbody>
              {endpoints.map((endpoint) => (
                <tr key={endpoint.id} className="border-t border-[var(--color-border-muted)]">
                  <td className="px-4 py-3"><MethodBadge method={endpoint.method} /></td>
                  <td className="px-4 py-3 font-mono text-[var(--color-text-primary)]">{endpoint.path}</td>
                  <td className="px-4 py-3 text-[var(--color-text-secondary)]">{endpoint.purpose}</td>
                  <td className="px-4 py-3">
                    <Button size="sm" variant="secondary" onClick={() => onOpenDoc(endpoint.id)}>
                      查看详情
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}

function StructuredApiDetailPage({
  docId,
  launchBaseUrl,
  authHeader,
  onOpenDoc,
}: {
  docId: Exclude<StructuredApiDocId, StructuredApiSectionId>
  launchBaseUrl: string
  authHeader: string
  onOpenDoc: (id: StructuredApiDocId) => void
}) {
  const endpoint = STRUCTURED_API_ENDPOINT_DOC_MAP[docId]

  return (
    <div className="space-y-6">
      <div>
        <Button variant="secondary" size="sm" onClick={() => onOpenDoc(endpoint.parentId)}>
          <ArrowLeft className="h-4 w-4" />
          返回接口列表
        </Button>
      </div>

      <section className="rounded-[24px] border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-5 shadow-[var(--shadow-sm)]">
        <div className="space-y-2">
          <div className="flex items-center gap-3 flex-wrap">
            <MethodBadge method={endpoint.method} />
            <p className="font-mono text-sm text-[var(--color-text-primary)]">{endpoint.path}</p>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-[var(--color-text-primary)]">
            {endpoint.label}
          </h1>
        </div>
      </section>

      <section className="space-y-3">
        <SectionTitle title="请求参数" />
        <FieldTable fields={endpoint.fields} />
      </section>

      {endpoint.requestExample && (
        <section className="space-y-3">
          <SectionTitle title="请求示例" />
          <LaunchDocsCodeBlock language={endpoint.requestExample.language} code={endpoint.requestExample.code({ launchBaseUrl, authHeader })} />
        </section>
      )}

      <section className="space-y-3">
        <SectionTitle title="状态码" />
        <ResponseCodeTable items={endpoint.responseCodes} />
      </section>

      {endpoint.responseExample && (
        <section className="space-y-3">
          <SectionTitle title="成功响应示例" />
          <LaunchDocsCodeBlock language={endpoint.responseExample.language} code={endpoint.responseExample.code({ launchBaseUrl, authHeader })} />
        </section>
      )}

      {endpoint.notes.length > 0 ? (
        <section className="rounded-2xl border border-amber-200 bg-amber-50/60 px-5 py-4 shadow-[var(--shadow-sm)]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-amber-700">注意</p>
          <ul className="mt-3 space-y-2 text-sm leading-6 text-[var(--color-text-secondary)]">
            {endpoint.notes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}

export function StructuredApiDocsPage({
  docId,
  launchBaseUrl,
  authHeader,
  onOpenDoc,
}: StructuredApiDocsPageProps) {
  if (isStructuredApiEndpointDocId(docId)) {
    return (
      <StructuredApiDetailPage
        docId={docId}
        launchBaseUrl={launchBaseUrl}
        authHeader={authHeader}
        onOpenDoc={onOpenDoc}
      />
    )
  }

  return (
    <StructuredApiSectionPage
      docId={docId}
      onOpenDoc={onOpenDoc}
    />
  )
}
