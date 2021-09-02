// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package upstart provides constants shared by remote and local tests.
package upstart

// Goal describes a job's goal. See Section 10.1.6.19, "initctl status", in the Upstart Cookbook.
type Goal string

// State describes a job's current state. See Section 4.1.2, "Job States", in the Upstart Cookbook.
type State string

const (
	// StartGoal indicates that a task or service job has been started.
	StartGoal Goal = "start"
	// StopGoal indicates that a task job has completed or that a service job has been manually stopped or has
	// a "stop on" condition that has been satisfied.
	StopGoal Goal = "stop"

	// WaitingState is the initial state for a job.
	WaitingState State = "waiting"
	// StartingState indicates that a job is about to start.
	StartingState State = "starting"
	// SecurityState indicates that a job is having its AppArmor security policy loaded.
	SecurityState State = "security"
	// TmpfilesState indicates that a job is having its temporary files set up.
	TmpfilesState State = "tmpfiles"
	// PreStartState indicates that a job's pre-start section is running.
	PreStartState State = "pre-start"
	// SpawnedState indicates that a job's script or exec section is about to run.
	SpawnedState State = "spawned"
	// PostStartState indicates that a job's post-start section is running.
	PostStartState State = "post-start"
	// RunningState indicates that a job is running (i.e. its post-start section has completed). It may not have a PID yet.
	RunningState State = "running"
	// PreStopState indicates that a job's pre-stop section is running.
	PreStopState State = "pre-stop"
	// StoppingState indicates that a job's pre-stop section has completed.
	StoppingState State = "stopping"
	// KilledState indicates that a job is about to be stopped.
	KilledState State = "killed"
	// PostStopState indicates that a job's post-stop section is running.
	PostStopState State = "post-stop"
)
