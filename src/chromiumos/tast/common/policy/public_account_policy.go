// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

// PublicAccountPolicy embeds a Policy and changes its scope to ScopePublicAccount.
// The PublicAccountPolicy is otherwise identical to the embedded policy.
type PublicAccountPolicy struct {
	Policy
}

// Scope returns ScopePublicAccount.
func (p *PublicAccountPolicy) Scope() Scope { return ScopePublicAccount }
