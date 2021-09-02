// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tracing contains shared functionalities for Perfetto system tracing tests.
package tracing

const (
	// TraceConfigFile is the data path of the trace config file in text proto format.
	TraceConfigFile = "perfetto/system_trace_cfg.pbtxt"

	// TBMTracedProbesConfigFile is the data path of the TBM traced probes config file in text proto format.
	TBMTracedProbesConfigFile = "perfetto/perfetto_tbm_traced_probes.pbtxt"

	// TracedJobName is the upstart job names of the Perfetto system tracing service daemon (traced).
	TracedJobName = "traced"

	// TracedProbesJobName is the upstart job name of the Perfetto system tracing probes (traced_probes).
	TracedProbesJobName = "traced_probes"

	// TraceProcessorAmd64 is the name of tps for architecture: amd64.
	TraceProcessorAmd64 = "trace_processor_shell-amd64"
	// TraceProcessorArm is the name of tps for architecture: arm.
	TraceProcessorArm = "trace_processor_shell-arm"
	// TraceProcessorArm64 is the name of tps for architecture: arm64.
	TraceProcessorArm64 = "trace_processor_shell-arm64"
)
