/**
 * Copyright 2026 The Crater Project Team, RAIDS-Lab
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import assert from 'node:assert/strict'
import test from 'node:test'

import {
  isBillingVisibleForAdmin,
  isBillingVisibleForUser,
  isJobBillingSupported,
} from './billing-visibility.ts'

test('job billing endpoints are only available for VCJob', () => {
  assert.equal(isJobBillingSupported('vcjobs'), true)
  assert.equal(isJobBillingSupported('aijobs'), false)
  assert.equal(isJobBillingSupported('spjobs'), false)
})

test('EMIAS never enables the unsupported job billing query', () => {
  const enabled =
    isJobBillingSupported('aijobs') &&
    isBillingVisibleForUser({ featureEnabled: true, active: true })

  assert.equal(enabled, false)
})

test('billing status visibility still distinguishes users and administrators', () => {
  const paused = { featureEnabled: true, active: false }
  assert.equal(isBillingVisibleForAdmin(paused), true)
  assert.equal(isBillingVisibleForUser(paused), false)
})
