import { describe, expect, it } from 'vitest'
import { specialistTaskStatusLabel, specialistTaskDeadlineTone, requiredStepSummary } from './presentation'

describe('specialist task presentation', () => {
  it('labels known task states', () => {
    expect(specialistTaskStatusLabel('pending')).toBe('待开始')
    expect(specialistTaskStatusLabel('submitted_pending_validation')).toBe('已提交待校验')
    expect(specialistTaskStatusLabel('appeal_in_review')).toBe('申诉中')
  })

  it('marks overdue deadlines as danger', () => {
    const past = new Date(Date.now() - 60_000).toISOString()
    expect(specialistTaskDeadlineTone(past)).toBe('danger')
  })

  it('summarizes required SOP steps', () => {
    expect(requiredStepSummary([
      { required: true, status: 'done' },
      { required: true, status: 'not_started' },
      { required: false, status: 'not_started' },
    ])).toBe('1/2')
  })
})
