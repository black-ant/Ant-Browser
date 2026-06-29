import { describe, expect, it } from 'vitest'
import type { RunRecord, ShopRunEvidence } from '../runEvidence'
import type { WorkspaceAuthorizedShop } from '../workspace/types'
import { evidenceForWorkbenchRow, shouldSuppressStaleFailure } from './rowState'

function shop(overrides: Partial<WorkspaceAuthorizedShop> = {}): WorkspaceAuthorizedShop {
  return {
    shopId: 'shop-1',
    shopName: '测试店铺',
    platformCode: 'alibaba',
    profileId: 'alibaba:shop-1',
    instanceId: '',
    sharedLoginStatus: 'ready',
    sharedLoginStatusLabel: '可自动登录',
    instanceRunning: false,
    profileExists: true,
    reclaimPending: false,
    coreReady: true,
    lastValidatedAt: '',
    lastOpenedAt: '',
    lastOpenFailureCode: '',
    lastOpenFailureMessage: '',
    lastOpenFailedAt: '',
    ...overrides,
  }
}

function run(overrides: Partial<RunRecord>): RunRecord {
  return {
    runId: 'run-1',
    taskId: 'task-1',
    shopId: 'shop-1',
    taskType: 'bind',
    status: 'failed',
    statusLabel: '失败',
    startedAt: '2026-06-03T09:41:38.885Z',
    finishedAt: '2026-06-03T09:41:47.435Z',
    profileId: 'alibaba:shop-1',
    runtime: null,
    bindSessionId: '',
    manualActionRequired: false,
    challengeType: '',
    failureCode: 'LOCAL_BRIDGE_FAILED',
    failureMessage: 'fetch failed',
    ...overrides,
  }
}

function evidence(overrides: Partial<ShopRunEvidence> = {}): ShopRunEvidence {
  return {
    latestOpen: null,
    latestCredential: null,
    latestValidation: null,
    latestFailure: null,
    activeRun: null,
    ...overrides,
  }
}

describe('workbench row state evidence', () => {
  it('suppresses old bind failures once shared login is ready', () => {
    const source = evidence({ latestFailure: run({ taskType: 'bind' }) })

    expect(shouldSuppressStaleFailure(shop(), source)).toBe(true)
    expect(evidenceForWorkbenchRow(shop(), source).latestFailure).toBeNull()
  })

  it('keeps failures while shared login is not ready', () => {
    const source = evidence({ latestFailure: run({ taskType: 'bind' }) })

    expect(shouldSuppressStaleFailure(shop({ sharedLoginStatus: 'validation_failed' }), source)).toBe(false)
    expect(evidenceForWorkbenchRow(shop({ sharedLoginStatus: 'validation_failed' }), source).latestFailure).toBe(source.latestFailure)
  })

  it('suppresses open failures older than a successful credential run', () => {
    const source = evidence({
      latestCredential: run({
        runId: 'run-credential-success',
        taskType: 'bind',
        status: 'succeeded',
        startedAt: '2026-06-05T09:18:00.000Z',
        finishedAt: '2026-06-05T09:19:00.000Z',
        failureCode: '',
        failureMessage: '',
      }),
      latestFailure: run({
        runId: 'run-open-failed',
        taskType: 'open',
        startedAt: '2026-06-03T09:41:38.885Z',
        finishedAt: '2026-06-03T09:41:47.435Z',
      }),
    })

    expect(shouldSuppressStaleFailure(shop(), source)).toBe(true)
    expect(evidenceForWorkbenchRow(shop(), source).latestFailure).toBeNull()
  })

  it('suppresses reported open failures older than a successful credential run', () => {
    const source = evidence({
      latestCredential: run({
        runId: 'run-credential-success',
        taskType: 'bind',
        status: 'succeeded',
        startedAt: '2026-06-05T09:18:00.000Z',
        finishedAt: '2026-06-05T09:19:00.000Z',
        failureCode: '',
        failureMessage: '',
      }),
    })

    expect(evidenceForWorkbenchRow(shop({
      lastOpenFailureCode: 'LOCAL_BRIDGE_FAILED',
      lastOpenFailureMessage: 'fetch failed',
      lastOpenFailedAt: '2026-06-03T09:41:47.435Z',
    }), source).latestFailure).toBeNull()
  })

  it('suppresses reported open failures older than the last successful validation timestamp', () => {
    const source = evidence()

    expect(evidenceForWorkbenchRow(shop({
      lastValidatedAt: '2026-06-09T09:20:00.000Z',
      lastOpenFailureCode: 'ANT_FINGERPRINT_CORE_REQUIRED',
      lastOpenFailureMessage: 'managed shop requires a fingerprint core',
      lastOpenFailedAt: '2026-06-09T09:10:00.000Z',
    }), source).latestFailure).toBeNull()
  })

  it('suppresses recoverable fingerprint-core open failures after the shop is ready', () => {
    const source = evidence()

    expect(evidenceForWorkbenchRow(shop({
      coreReady: true,
      sharedLoginStatus: 'ready',
      lastOpenFailureCode: 'ANT_FINGERPRINT_CORE_REQUIRED',
      lastOpenFailureMessage: 'managed shop requires a fingerprint core',
      lastOpenFailedAt: '2026-06-06T01:25:42.415Z',
    }), source).latestFailure).toBeNull()
  })

  it('suppresses reported open failures older than a successful open', () => {
    const source = evidence()

    expect(evidenceForWorkbenchRow(shop({
      lastOpenedAt: '2026-06-09T09:10:00.000Z',
      lastOpenFailureCode: 'LOCAL_BRIDGE_FAILED',
      lastOpenFailureMessage: 'Maka Browser Runtime 不可达',
      lastOpenFailedAt: '2026-06-09T09:00:00.000Z',
    }), source).latestFailure).toBeNull()
  })

  it('keeps reported open failures newer than the last successful open', () => {
    const source = evidence()

    expect(evidenceForWorkbenchRow(shop({
      lastOpenedAt: '2026-06-09T09:00:00.000Z',
      lastOpenFailureCode: 'LOCAL_BRIDGE_FAILED',
      lastOpenFailureMessage: 'Maka Browser Runtime 不可达',
      lastOpenFailedAt: '2026-06-09T09:10:00.000Z',
    }), source).latestFailure?.failureCode).toBe('LOCAL_BRIDGE_FAILED')
  })
})
