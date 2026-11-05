const fs = require('fs')
const {
  DEFAULT_EXCLUDED_DOMAINS,
  normalizeInt,
  normalizeText,
  normalizeDomainList,
  buildDefaultQuery,
  buildFallbackQueries,
  buildSearchQuery,
  buildSearchURL,
  splitSnippet,
  evaluateNewsItem,
  formatReport,
  pickBestAttempt,
} = require('./news-query-utils.cjs')

module.exports.run = async ({ launch, connect, openPage, selector, params, log, artifact }) => {
  const timeout = normalizeInt(params.timeoutMs, 30000, 1000, 120000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 1500, 0, 10000)
  const limit = normalizeInt(params.limit, 10, 1, 50)
  const maxPages = normalizeInt(params.maxPages, 3, 1, 5)
  const baseQuery = normalizeText(params.query) || buildDefaultQuery(params.keyword)
  const excludedDomains = normalizeDomainList(params.excludeDomains).length > 0
    ? normalizeDomainList(params.excludeDomains)
    : DEFAULT_EXCLUDED_DOMAINS
  const outputFileName = normalizeText(params.outputFileName) || 'news-results.txt'
  const scanLimit = Math.max(10, Math.min(20, limit * 2))
  const startUrls = Array.isArray(params.startUrls) && params.startUrls.length > 0
    ? params.startUrls
    : undefined

  const session = await launch({
    selector,
    startUrls,
    skipDefaultStartUrls: true,
  })

  const connection = await connect(session)
  const opened = await openPage(connection, {
    reuseCurrentPage: true,
    bringToFront: true,
    timeoutMs: timeout,
  })
  const page = opened.page
  const closeRunnerPage = async function () {
    if (!page.isClosed()) {
      await page.close().catch(function () {})
    }
  }

  const searchCandidates = buildFallbackQueries(params.keyword, baseQuery)
  const minAcceptedCount = Math.min(limit, Math.max(2, Math.ceil(limit * 0.2)))
  const minDistinctHostCount = Math.min(3, minAcceptedCount)
  let bestAttempt = null

  try {
    for (const candidateQuery of searchCandidates) {
      const searchQuery = buildSearchQuery(candidateQuery, excludedDomains)
      const normalizedItems = []
      const seenUrls = new Set()
      let scannedPageCount = 0
      let firstSearchUrl = ''

      for (let pageIndex = 0; pageIndex < maxPages; pageIndex += 1) {
        const firstResultIndex = pageIndex * 10 + 1
        const searchUrl = buildSearchURL(searchQuery, params.timeRange, firstResultIndex)

        try {
          await page.goto(searchUrl, {
            waitUntil: 'domcontentloaded',
            timeout,
          })
          await page.waitForSelector('li.b_algo', { timeout })
        } catch (error) {
          if (pageIndex > 0 && normalizedItems.length > 0) {
            break
          }
          throw error
        }

        if (waitAfterLoadMs > 0) {
          await page.waitForTimeout(waitAfterLoadMs)
        }

        if (!firstSearchUrl) {
          firstSearchUrl = page.url()
        }

        const pageItems = await page.$$eval('li.b_algo', function (nodes, maxItems) {
          const clean = function (value) {
            return String(value || '').replace(/\s+/g, ' ').trim()
          }

          return nodes
            .slice(0, maxItems)
            .map(function (node) {
              const titleLink = node.querySelector('h2 a')
              const title = clean(titleLink && titleLink.textContent)
              const url = titleLink ? titleLink.href : ''
              const sourceNode = node.querySelector('.tptt')
              const source = clean(sourceNode && sourceNode.textContent)
              const citeNode = node.querySelector('.b_attribution cite')
              const cite = clean(citeNode && citeNode.textContent)
              const snippetNode = node.querySelector('.b_caption p')
              const snippet = clean(snippetNode && snippetNode.textContent)

              if (!title) {
                return null
              }

              return {
                title,
                url,
                source: source || cite,
                snippet,
              }
            })
            .filter(Boolean)
        }, scanLimit)

        let appendedCount = 0
        for (const item of pageItems) {
          const dedupeKey = normalizeText(item.url)
          if (!dedupeKey || seenUrls.has(dedupeKey)) {
            continue
          }

          seenUrls.add(dedupeKey)
          normalizedItems.push(
            evaluateNewsItem(
              Object.assign(
                {
                  rank: normalizedItems.length + 1,
                },
                item,
                splitSnippet(item.snippet)
              )
            )
          )
          appendedCount += 1
        }

        scannedPageCount += 1
        if (appendedCount === 0 || pageItems.length < 8) {
          break
        }
      }

      const acceptedItems = normalizedItems.filter(function (item) {
        return item.qualityAccepted
      }).slice(0, limit)
      const rejectedItems = normalizedItems.filter(function (item) {
        return !item.qualityAccepted
      })
      const distinctHostCount = new Set(
        acceptedItems
          .map(function (item) {
            return item.hostname
          })
          .filter(Boolean)
      ).size

      log('searchQuery', searchQuery)
      log('rawItemCount', normalizedItems.length)
      log('acceptedItemCount', acceptedItems.length)
      log('rejectedItemCount', rejectedItems.length)
      log('distinctHostCount', distinctHostCount)
      log('scannedPageCount', scannedPageCount)

      bestAttempt = pickBestAttempt(bestAttempt, {
        baseQuery: candidateQuery,
        searchQuery: searchQuery,
        searchUrl: firstSearchUrl || page.url(),
        rawItems: normalizedItems,
        acceptedItems: acceptedItems,
        rejectedItems: rejectedItems,
        distinctHostCount: distinctHostCount,
        scannedPageCount: scannedPageCount,
      })

      if (acceptedItems.length >= minAcceptedCount && distinctHostCount >= minDistinctHostCount) {
        break
      }
    }
  } catch (error) {
    await closeRunnerPage()
    throw error
  }

  if (!bestAttempt || bestAttempt.rawItems.length === 0) {
    await closeRunnerPage()
    throw new Error('未抓到新闻搜索结果，当前页面: ' + page.url())
  }

  const normalizedItems = bestAttempt.rawItems
  const acceptedItems = bestAttempt.acceptedItems
  const rejectedItems = bestAttempt.rejectedItems
  const distinctHostCount = bestAttempt.distinctHostCount
  const searchUrl = bestAttempt.searchUrl
  const scannedPageCount = bestAttempt.scannedPageCount || 1

  const outputName = outputFileName.toLowerCase().endsWith('.txt')
    ? outputFileName
    : outputFileName + '.txt'
  const outputPath = artifact(outputName)
  const reportText = formatReport(acceptedItems, {
    query: bestAttempt.baseQuery,
    generatedAt: new Date().toISOString(),
    searchUrl: searchUrl,
    rawCount: normalizedItems.length,
    rejectedItems: rejectedItems,
  })
  fs.writeFileSync(outputPath, reportText, 'utf8')

  let screenshotPath = ''
  if (params.captureScreenshot === true) {
    screenshotPath = artifact('news-search.png')
    await page.screenshot({
      path: screenshotPath,
      fullPage: true,
    })
  }

  log('outputPath', outputPath)
  await closeRunnerPage()

  if (acceptedItems.length < minAcceptedCount || distinctHostCount < minDistinctHostCount) {
    return {
      ok: false,
      summary: '新闻结果质量不足，仅 ' + acceptedItems.length + '/' + normalizedItems.length + ' 条通过校验',
      error: '搜索结果更像普通搜索、问答页或聚合页，未达到新闻抓取标准',
      query: bestAttempt.baseQuery,
      searchQuery: bestAttempt.searchQuery,
      searchUrl: searchUrl,
      outputPath,
      screenshotPath,
      rawItemCount: normalizedItems.length,
      itemCount: acceptedItems.length,
      rejectedCount: rejectedItems.length,
      distinctHostCount: distinctHostCount,
      scannedPageCount: scannedPageCount,
      firstTitle: acceptedItems[0] ? acceptedItems[0].title : '',
    }
  }

  return {
    ok: true,
    summary: '已筛出 ' + acceptedItems.length + ' 条有效新闻并写入 TXT',
    query: bestAttempt.baseQuery,
    searchQuery: bestAttempt.searchQuery,
    searchUrl: searchUrl,
    outputPath,
    screenshotPath,
    rawItemCount: normalizedItems.length,
    itemCount: acceptedItems.length,
    rejectedCount: rejectedItems.length,
    distinctHostCount: distinctHostCount,
    scannedPageCount: scannedPageCount,
    firstTitle: acceptedItems[0] ? acceptedItems[0].title : '',
  }
}
