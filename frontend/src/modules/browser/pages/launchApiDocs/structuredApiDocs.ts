export type {
  StructuredApiDocId,
  StructuredApiEndpointDoc,
  StructuredApiExample,
  StructuredApiExampleContext,
  StructuredApiField,
  StructuredApiMethod,
  StructuredApiResponseCode,
  StructuredApiSectionDoc,
  StructuredApiSectionId,
} from './structuredApiDocs.types'
import type { StructuredApiDocId, StructuredApiEndpointDoc, StructuredApiSectionDoc, StructuredApiSectionId } from './structuredApiDocs.types'
import { AUTOMATION_API_ENDPOINT_DOCS } from './structuredApiDocs.automationEndpoints'
import { PROFILE_API_ENDPOINT_DOCS } from './structuredApiDocs.profileEndpoints'
import { RUNTIME_API_ENDPOINT_DOCS } from './structuredApiDocs.runtimeEndpoints'
import { STRUCTURED_API_SECTION_DOCS } from './structuredApiDocs.sections'

export { STRUCTURED_API_SECTION_DOCS } from './structuredApiDocs.sections'

export const STRUCTURED_API_ENDPOINT_DOCS: StructuredApiEndpointDoc[] = [
  ...PROFILE_API_ENDPOINT_DOCS,
  ...RUNTIME_API_ENDPOINT_DOCS,
  ...AUTOMATION_API_ENDPOINT_DOCS,
]

export const STRUCTURED_API_SECTION_DOC_MAP = Object.fromEntries(
  STRUCTURED_API_SECTION_DOCS.map((doc) => [doc.id, doc]),
) as Record<StructuredApiSectionId, StructuredApiSectionDoc>

export const STRUCTURED_API_ENDPOINT_DOC_MAP = Object.fromEntries(
  STRUCTURED_API_ENDPOINT_DOCS.map((doc) => [doc.id, doc]),
) as Record<Exclude<StructuredApiDocId, StructuredApiSectionId>, StructuredApiEndpointDoc>

const STRUCTURED_API_DOC_IDS = new Set<StructuredApiDocId>([
  ...STRUCTURED_API_SECTION_DOCS.map((doc) => doc.id),
  ...STRUCTURED_API_ENDPOINT_DOCS.map((doc) => doc.id),
])

export function isStructuredApiDocId(id: string): id is StructuredApiDocId {
  return STRUCTURED_API_DOC_IDS.has(id as StructuredApiDocId)
}

export function isStructuredApiEndpointDocId(id: string): id is Exclude<StructuredApiDocId, StructuredApiSectionId> {
  return id in STRUCTURED_API_ENDPOINT_DOC_MAP
}

export function getStructuredApiParentDocId(id: StructuredApiDocId): StructuredApiSectionId {
  if (id in STRUCTURED_API_SECTION_DOC_MAP) {
    return id as StructuredApiSectionId
  }
  return STRUCTURED_API_ENDPOINT_DOC_MAP[id as Exclude<StructuredApiDocId, StructuredApiSectionId>].parentId
}

export function getStructuredApiHiddenDocItems() {
  return STRUCTURED_API_ENDPOINT_DOCS.map((doc) => ({
    id: doc.id,
    label: doc.label,
    summary: doc.purpose,
    content: '',
    parentId: doc.parentId,
    hidden: true,
  }))
}

export function getStructuredApiSectionEndpoints(sectionId: StructuredApiSectionId) {
  return STRUCTURED_API_ENDPOINT_DOCS.filter((doc) => doc.parentId === sectionId)
}
