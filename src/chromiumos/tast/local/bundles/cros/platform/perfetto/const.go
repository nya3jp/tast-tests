// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perfetto contains shared functionalities for Perfetto system tracing tests.
package perfetto

const (
	// TraceConfigFile is the data path of the trace config file in text proto format.
	TraceConfigFile = "perfetto/system_trace_cfg.pbtxt"

	// TBMTracedProbesConfigFile is the data path of the TBM traced probes config file in text proto format.
	TBMTracedProbesConfigFile = "perfetto/perfetto_tbm_traced_probes.pbtxt"

	// TracedJobName is the upstart job names of the Perfetto system tracing service daemon (traced).
	TracedJobName = "traced"

	// TracedProbesJobName is the upstart job name of the Perfetto system tracing probes (traced_probes).
	TracedProbesJobName = "traced_probes"
)
