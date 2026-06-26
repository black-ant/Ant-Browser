export type SpecialistTaskStatus =
  | 'pending'
  | 'in_progress'
  | 'submitted_pending_validation'
  | 'appeal_in_review'
  | 'rejected_rework'
  | 'completed'
  | 'validation_failed_penalty'
  | 'overdue'
  | 'cancelled'

export type SpecialistTaskPriority = 'critical' | 'high' | 'medium' | 'low' | string

export type SpecialistEvidenceType =
  | 'screenshot'
  | 'backend_url'
  | 'product_url'
  | 'text_note'
  | 'operation_summary'
  | string

export interface SpecialistTaskSopStep {
  id: string
  taskId: string
  stepId: string
  actionCode: string
  title: string
  description: string
  required: boolean
  evidenceTypes: SpecialistEvidenceType[]
  status: string
  evidenceRefs: string[]
  operatorNote: string
  createdAt: string
  updatedAt: string
}

export interface SpecialistTaskEvidenceRecord {
  id: string
  taskId: string
  stepId: string | null
  evidenceType: SpecialistEvidenceType
  payload: Record<string, unknown>
  submittedBy: string
  createdAt: string
}

export interface SpecialistTaskRecord {
  id: string
  shopId: string
  shopName: string
  title: string
  description: string
  sourceType: string
  priority: SpecialistTaskPriority
  status: SpecialistTaskStatus | string
  timelineBlockCode: string
  timelineBlockLabel: string
  assigneeUserId: string
  assigneeName: string
  supervisorUserId: string
  supervisorName: string
  deadlineAt: string | null
  pauseStartedAt: string | null
  pausedSeconds: number
  validationResult: string | null
  metadata: Record<string, unknown>
  anomalySignalId: string | null
  anomalySignalType: string | null
  anomalySignalSourceRef: string | null
  anomalySignalSourceTier: string | null
  anomalySignalSnapshot: Record<string, unknown> | null
  createdAt: string
  updatedAt: string
  sopSteps: SpecialistTaskSopStep[]
  evidenceRecords: SpecialistTaskEvidenceRecord[]
}

export interface SpecialistTaskPagination {
  page: number
  pageSize: number
  total: number
}

export interface SpecialistTaskSummary {
  total: number
  pending: number
  inProgress: number
  submittedPendingValidation: number
  appealInReview: number
  overdue: number
  completed: number
}

export interface SpecialistTaskListResponse {
  items: SpecialistTaskRecord[]
  pagination: SpecialistTaskPagination
  summary: SpecialistTaskSummary
}

export interface SpecialistTaskListQuery {
  page?: number
  pageSize?: number
  status?: string
}

export interface UpdateSpecialistTaskSopStepPayload {
  status: 'not_started' | 'in_progress' | 'done' | string
  operatorNote?: string
  evidenceRefs?: string[]
}

export interface SubmitSpecialistTaskEvidencePayload {
  stepId: string
  evidenceType: SpecialistEvidenceType
  payload: Record<string, unknown>
}

export interface SubmitSpecialistTaskPayload {
  summary?: string
}

export interface AppealSpecialistTaskPayload {
  reason: string
}

export interface BlockSpecialistTaskPayload {
  reasonCode: string
  reasonText: string
}

export interface SpecialistTaskMutationResponse {
  task: SpecialistTaskRecord
  evidenceId?: string
  appealId?: string
}
