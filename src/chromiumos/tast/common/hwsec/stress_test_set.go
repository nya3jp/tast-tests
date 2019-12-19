// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
	a9n "chromiumos/tast/local/hwsec/attestation"
)

type attestationFlow interface {
	// IsEnrolled returns the flag to indicate if the DUT is
	// enrolled and any encounted error during the opeation.
	IsEnrolled(ctx context.Context) (bool, error)
	// Creates an enroll request that is sent to the corresponding pca server of |pcaType|
	// later, and any error encountered during the operation.
	CreateEnrollRequest(ctx context.Context, pcaType PCAType) (string, error)
	// Finishes the enroll with |resp| from pca server of |pcaType|. Returns any
	// encountered error during the operation.
	FinishEnroll(ctx context.Context, pcaType PCAType, resp string) error
	// Creates a certificate request that is sent to the corresponding pca server
	// of |pcaType| later, and any error encountered during the operation.
	CreateCertRequest(
		ctx context.Context,
		pcaType PCAType,
		profile apb.CertificateProfile,
		username string,
		origin string) (string, error)
	// Finishes the certified key creation with |resp| from PCA server. Returns any encountered error during the operation.
	FinishCertRequest(ctx context.Context, response string, username string, label string) error
}

// StressTaskEnroll is a task piece that runs through the enrollment process.
type StressTaskEnroll struct {
	af attestationFlow
}

// NewStressTaskEnroll creates a new StressTaskEnroll, where |u| provides underlying implementation of dbus methods.
func NewStressTaskEnroll(af attestationFlow) *StressTaskEnroll {
	return &StressTaskEnroll{af}
}

// RunTask implements the one of StressTaskRunner.
func (r *StressTaskEnroll) RunTask(ctx context.Context) error {
	// Don't always sanity-check the preparation; the following call fails in that case anyway
	req, err := r.af.CreateEnrollRequest(ctx, DefaultPCA)
	if err != nil {
		return errors.Wrap(err, "failed to create enroll request")
	}
	resp, err := a9n.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		return errors.Wrap(err, "failed to send request to CA")
	}
	err = r.af.FinishEnroll(ctx, DefaultPCA, resp)
	if err != nil {
		return errors.Wrap(err, "failed to finish enrollment")
	}
	isEnrolled, err := r.af.IsEnrolled(ctx)
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
	af              attestationFlow
	username, label string
}

// NewStressTaskCert creates a new StressTaskCert, where |u| provides underlying implementation of dbus methods.
func NewStressTaskCert(af attestationFlow, username, label string) *StressTaskCert {
	return &StressTaskCert{af, username, label}
}

// RunTask implements the one of StressTaskRunner.
func (r *StressTaskCert) RunTask(ctx context.Context) error {
	req, err := r.af.CreateCertRequest(
		ctx,
		DefaultPCA,
		a9n.DefaultCertProfile,
		r.username,
		a9n.DefaultCertOrigin)
	if err != nil {
		return errors.Wrap(err, "failed to create certificate request")
	}
	resp, err := a9n.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/sign")
	if err != nil {
		return errors.Wrap(err, "failed to send request to CA")
	}
	if len(resp) == 0 {
		return errors.New("unexpected empty cert")
	}
	err = r.af.FinishCertRequest(ctx, resp, r.username, r.label)
	if err != nil {
		return errors.Wrap(err, "failed to finish cert request")
	}
	return nil
}

type vaultController interface {
	// Checks if any currently mounted vault; If the operation succeeds, then error will be nil, and the bool will contain if any user vault is mounted (true if any vault is mounted). Otherwise, an error is returned.
	IsMounted(ctx context.Context) (bool, error)

	// Unmounts the vault of |username|
	Unmount(ctx context.Context, username string) (bool, error)

	// Creates the vault for |username| if not exist.
	CreateVault(ctx context.Context, username string, password string) (bool, error)

	// Checks the vault for |username|.
	CheckVault(ctx context.Context, username string, password string) (bool, error)

	// Removes the vault of |username|.
	RemoveVault(ctx context.Context, username string) (bool, error)
}

type stressTaskMount struct {
	vc       vaultController
	username string
	passwd   string
}

//NewStressTaskMount creates a new stressTaskMount.
func NewStressTaskMount(vc vaultController, username, passwd string) *stressTaskMount {
	return &stressTaskMount{vc, username, passwd}
}

// RunTask implements the one of StressTaskRunner.
func (r *stressTaskMount) RunTask(ctx context.Context) error {
	username := r.username
	passwd := r.passwd
	result, err := r.vc.CreateVault(ctx, username, passwd)
	if err != nil {
		return errors.Wrap(err, "error during create vault")
	} else if !result {
		return errors.New("failed to create vault")
	}
	result, err = r.vc.Unmount(ctx, username)
	if err != nil {
		return errors.Wrap(err, "error unmounting user")
	}
	if !result {
		return errors.Wrap(err, "failed to unmount user")
	}
	result, err = r.vc.RemoveVault(ctx, username)
	if err != nil {
		return errors.Wrap(err, "error removing vault")
	}
	if !result {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// stressTaskTakeOwnership creates a new StressTaskCert, where |utility| provides underlying implementation of dbus methods.
type stressTaskTakeOwnership struct {
	helper *Helper
}

// NewStressTaskTakeOwnership creates a new stressTaskTakeOwnership, where |u| provides underlying implementation of dbus methods.
func NewStressTaskTakeOwnership(h *Helper) *stressTaskTakeOwnership {
	return &stressTaskTakeOwnership{h}
}

// RunTask implements the one of StressTaskRunner.
func (r *stressTaskTakeOwnership) RunTask(ctx context.Context) error {
	return r.helper.EnsureTPMIsReady(ctx, DefaultTakingOwnershipTimeout)
}

type stressTaskCheckKey struct {
	vc       vaultController
	username string
	passwd   string
}

// NewStressTaskCheckKey creates a new stressTaskCheckKey, where |u| provides underlying implementation of dbus methods.
func NewStressTaskCheckKey(vc vaultController, username, passwd string) *stressTaskCheckKey {
	return &stressTaskCheckKey{vc, username, passwd}
}

// RunTask implements the one of StressTaskRunner.
func (r *stressTaskCheckKey) RunTask(ctx context.Context) error {
	result, err := r.vc.CheckVault(ctx, r.username, r.passwd)
	if err != nil {
		return errors.Wrap(err, "error checking vault")
	}
	if !result {
		return errors.New("Failed to check vault")
	}
	return nil
}
