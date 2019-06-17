// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"chromiumos/tast/errors"
	a9n "chromiumos/tast/local/hwsec/attestation"
)

// StressTaskEnroll is a task piece that runs through the enrollment process.
type StressTaskEnroll struct {
	utility Utility
}

// NewStressTaskEnroll creates a new StressTaskEnroll, where |u| provides underlying implementation of dbus methods.
func NewStressTaskEnroll(u Utility) *StressTaskEnroll {
	return &StressTaskEnroll{u}
}

// RunTask implements the one of StressTaskRunner.
func (r *StressTaskEnroll) RunTask() error {
	// Don't always sanity-check the preparation; the following call fails in that case anyway
	req, err := r.utility.CreateEnrollRequest(a9n.DefaultACAType)
	if err != nil {
		return errors.Wrap(err, "failed to create enroll request")
	}
	resp, err := a9n.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		return errors.Wrap(err, "failed to send request to CA")
	}
	err = r.utility.FinishEnroll(a9n.DefaultACAType, resp)
	if err != nil {
		return errors.Wrap(err, "failed to finish enrollment")
	}
	isEnrolled, err := r.utility.IsEnrolled()
	if err != nil {
		return errors.Wrap(err, "failed to get enrollment status")
	}
	if !isEnrolled {
		return errors.New("inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	return nil
}

// StressTaskCert is a task piece that runs through the cert process.
type StressTaskCert struct {
	utility         Utility
	username, label string
}

// NewStressTaskCert creates a new StressTaskCert, where |u| provides underlying implementation of dbus methods.
func NewStressTaskCert(u Utility, username, label string) *StressTaskCert {
	return &StressTaskCert{u, username, label}
}

// RunTask implements the one of StressTaskRunner.
func (r *StressTaskCert) RunTask() error {
	req, err := r.utility.CreateCertRequest(
		a9n.DefaultACAType,
		a9n.DefaultCertProfile,
		r.username,
		a9n.DefaultCertOrigin)
	if err != nil {
		return errors.Wrap(err, "failed to create certificate request")
	}
	resp, err := a9n.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/sign")
	if err != nil {
		return errors.Wrap(err, "failed to send request to CA")
	}
	if len(resp) == 0 {
		return errors.New("unexpected empty cert")
	}
	err = r.utility.FinishCertRequest(resp, r.username, r.label)
	if err != nil {
		return errors.Wrap(err, "failed to finish cert request")
	}
	return nil
}

// StressTaskMount creates a new StressTaskCert, where |utility| provides underlying implementation of dbus methods.
type StressTaskMount struct {
	utility  Utility
	username string
	passwd   string
}

// RunTask implements the one of StressTaskRunner.
func (r *StressTaskMount) RunTask() error {
	username := r.username
	passwd := r.passwd
	utility := r.utility
	result, err := utility.CreateVault(username, passwd)
	if err != nil {
		return errors.Wrap(err, "error during create vault")
	} else if !result {
		return errors.New("failed to create vault")
	}
	result, err = utility.Unmount(username)
	if err != nil {
		return errors.Wrap(err, "error unmounting user")
	}
	if !result {
		return errors.Wrap(err, "failed to unmount user")
	}
	result, err = utility.RemoveVault(username)
	if err != nil {
		return errors.Wrap(err, "error removing vault")
	}
	if !result {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}
