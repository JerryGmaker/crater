import type { IBillingStatus } from '@/services/api/system-config'

type BillingStatusLike = Pick<IBillingStatus, 'featureEnabled' | 'active'> | null | undefined
type JobAPIPath = 'vcjobs' | 'aijobs' | 'spjobs'

export function isJobBillingSupported(jobAPIPath: JobAPIPath) {
  return jobAPIPath === 'vcjobs'
}

export function isBillingVisibleForAdmin(status: BillingStatusLike) {
  return Boolean(status?.featureEnabled)
}

export function isBillingVisibleForUser(status: BillingStatusLike) {
  return Boolean(status?.featureEnabled && status?.active)
}

export function isBillingVisible(status: BillingStatusLike, audience: 'admin' | 'user') {
  return audience === 'admin' ? isBillingVisibleForAdmin(status) : isBillingVisibleForUser(status)
}
