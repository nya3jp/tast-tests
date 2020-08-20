// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// SvcLoginReusePre implements New and Close for precondition.
type SvcLoginReusePre struct {
	// S is the service state.
	S *testing.ServiceState
	// CR is the chrome instance.
	CR *Chrome

	// outDir is the temporary test output directory.
	outDir string
}

// PrePrepare prepares the procondition.
func (p *SvcLoginReusePre) PrePrepare(ctx context.Context) error {
	if p.CR != nil {
		//TODO: check the current cr usability
		return nil
	}

	var userID, userPasswd string
	var ok bool
	if userID, ok = p.S.Var("userID"); !ok {
		return errors.New("UserID variable is not set")
	}
	if userPasswd, ok = p.S.Var("userPasswd"); !ok {
		return errors.New("userPasswd variable is not set")
	}
	var opts []Option // Options that should be passed to New.
	opts = append(
		opts, KeepState(),
		ARCSupported(),
		ReuseLogin(),
		GAIALogin(),
		Auth(userID, userPasswd, ""))

	cr, err := New(ctx, opts...)
	if err != nil {
		return err
	}
	p.CR = cr
	return nil
}

// PreClose closes the precondition resources.
func (p *SvcLoginReusePre) PreClose(ctx context.Context) error {
	// Remove service output directory. It's the gRPC client's responsibility to retrieve the logs.
	if p.outDir != "" {
		os.RemoveAll(p.outDir)
	}
	var err error
	if p.CR != nil {
		err = p.CR.Close(ctx)
		p.CR = nil
	}

	return err
}

// MakeOutDir creates and assigns a temporary directory for service run.
// It also ensures that the dir is accessible to all users. The returned boolean created
// indicates whether a new directory was created.
// It also stores the directory in the internal structure for late usage.
func (p *SvcLoginReusePre) MakeOutDir(ctx context.Context, outDir string) (created bool, err error) {
	defer func() {
		if err != nil && created {
			os.RemoveAll(outDir)
			created = false
		}
	}()

	if outDir == "" {
		return false, errors.New("OutDir is not assigned")
	}
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return false, err
		}
		created = true
	} else if err != nil {
		return false, err
	}

	// Make the directory traversable in case a test wants to write a file as another user.
	// (Note that we can't guarantee that all the parent directories are also accessible, though.)
	if err := os.Chmod(outDir, 0755); err != nil {
		return created, err
	}
	p.outDir = outDir
	return created, nil
}

// OutDir returns the outDir.
func (p *SvcLoginReusePre) OutDir() string {
	return p.outDir
}
