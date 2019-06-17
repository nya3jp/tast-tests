// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import ()

type CryptohomClient struct {
}

func (client CryptohomClient) IsTpmReady() error {

}
func (client CryptohomClient) EnsureTpmIsNotOwned() error                {}
func (client CryptohomClient) IsTpmReady() error                         {}
func (client CryptohomClient) ClearOwnership() error                     {}
func (client CryptohomClient) Reboot() error                             {}
func (client CryptohomClient) TakeOwnership() error                      {}
func (client CryptohomClient) WaitForTpmisReady() error                  {}
func (client CryptohomClient) CreateEnrollRequest() (string, error)      {}
func (client CryptohomClient) FinishEnroll() error                       {}
func (client CryptohomClient) CreateCertificateRequest() (string, error) {}
func (client CryptohomClient) FinishCertificateRequest() error           {}
func (client CryptohomClient) SignChallenge() (string, error)            {}
