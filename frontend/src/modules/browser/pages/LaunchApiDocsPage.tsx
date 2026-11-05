import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useLaunchContext } from '../hooks/useLaunchContext'
import {
  DOC_GROUPS,
  findDocById,
  getDefaultDoc,
  getAdjacentDocs,
  renderDocWithLaunchContext,
} from './launchApiDocs/catalog'
import { LaunchDocsFlowPage } from './launchApiDocs/LaunchDocsFlowPage'
import { LaunchDocsLayout } from './launchApiDocs/LaunchDocsLayout'
import { LaunchDocsMarkdownContent } from './launchApiDocs/LaunchDocsMarkdownContent'
import { LaunchDocsPager } from './launchApiDocs/LaunchDocsPager'
import { LaunchDocsSidebar } from './launchApiDocs/LaunchDocsSidebar'
import { StructuredApiDocsPage } from './launchApiDocs/StructuredApiDocsPage'
import {
  getStructuredApiParentDocId,
  isStructuredApiDocId,
  isStructuredApiEndpointDocId,
  type StructuredApiDocId,
} from './launchApiDocs/structuredApiDocs'

export function LaunchApiDocsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const firstDoc = getDefaultDoc()
  const [activeId, setActiveId] = useState(firstDoc.id)
  const { launchBaseUrl, apiAuth } = useLaunchContext()

  const activeDoc = findDocById(activeId) || firstDoc
  const { previous, next } = isStructuredApiEndpointDocId(activeDoc.id) ? { previous: null, next: null } : getAdjacentDocs(activeDoc.id)
  const sidebarActiveId = isStructuredApiDocId(activeDoc.id) ? getStructuredApiParentDocId(activeDoc.id) : activeDoc.id

  const selectDoc = (id: string, syncURL: boolean) => {
    const doc = findDocById(id)
    if (!doc) {
      return false
    }

    setActiveId(doc.id)
    if (syncURL) {
      setSearchParams({ doc: doc.id })
    }
    return true
  }

  useEffect(() => {
    const requestedDoc = searchParams.get('doc')?.trim() || ''
    if (!requestedDoc || requestedDoc === activeId) {
      return
    }

    if (!selectDoc(requestedDoc, false)) {
      setSearchParams({ doc: firstDoc.id })
    }
  }, [activeId, firstDoc.id, searchParams, setSearchParams])

  const renderedContent = renderDocWithLaunchContext(activeDoc.content, launchBaseUrl, apiAuth.header)

  return (
    <LaunchDocsLayout
      sidebar={(
        <LaunchDocsSidebar
          groups={DOC_GROUPS}
          activeId={sidebarActiveId}
          onSelect={(id) => {
            void selectDoc(id, true)
          }}
        />
      )}
      header={null}
      content={(
        <div className="space-y-4">
          {activeDoc.id === 'tutorial-flow'
            ? <LaunchDocsFlowPage baseUrl={launchBaseUrl} />
            : isStructuredApiDocId(activeDoc.id)
              ? (
                <StructuredApiDocsPage
                  docId={activeDoc.id as StructuredApiDocId}
                  launchBaseUrl={launchBaseUrl}
                  authHeader={apiAuth.header}
                  onOpenDoc={(id) => {
                    void selectDoc(id, true)
                  }}
                />
              )
              : <LaunchDocsMarkdownContent content={renderedContent} docId={activeDoc.id} />}
          <LaunchDocsPager
            previous={previous}
            next={next}
            onSelect={(id) => {
              void selectDoc(id, true)
            }}
          />
        </div>
      )}
    />
  )
}
