// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package verifier is a framework for running verification function in parallel
// to the actual test with option to re-run verification funtion in a loop until
// the primary test is finished.
//
// Usage:
// verF := func() (verifier.ResultType, error) {
//   res := any_test_function_desirable(additional_params)
//   return verifier.ResultType{Data: res, Timestamp: time.Now()}, nil
// }
// vf := verifier.NewVerifier(ctx, verF) // This only creates framework.
// defer vf.Finish() // This destroys framework.
// (...)
// vf.StartJob() // This triggers starting verification loop.
// (...test...)
// results, err := vf.StopJob()
// (analyze resutls slice)
package verifier
