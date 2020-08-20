// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package comm contains trace_replay application host <-> guest communication protocol structures.
package comm

const (
	// TestResultSuccess means all the tests in a test group were completed successfully
	TestResultSuccess = "Success"
	// TestResultFailure means one or several tests in the group encountered a failure
	TestResultFailure = "Failure"

	// ProtocolVersion defines the current version of the communictation protocol
	ProtocolVersion = 1
)

// VersionInfo is used as a continer for the protocol version information
type VersionInfo struct {
	ProtocolVersion uint32 `json:"ProtocolVersion,string"`
}

// ProxyServerInfo is used as a container for a proxy server information
type ProxyServerInfo struct {
	URL string `json:"URL"`
}

// RepositoryInfo is used as a container for the information about GC storage repository
type RepositoryInfo struct {
	RootURL string `json:"RootURL"`
	Version uint32 `json:"Version,string"`
}

// SystemInfo is used as a container for the host related information
type SystemInfo struct {
	Board           string `json:"Board"`
	Chipset         string `json:"Chipset"`
	Model           string `json:"Model"`
	ChromeOSVersion string `json:"ChromeOSVersion"`
}

// TestGroupConfig struct is a part of host->guest communication protocol and it used
// to define a trace replay test group configuration as well as a container to deliver
// the required host environment related information inside the guest
type TestGroupConfig struct {
	Labels      []string        `json:"Labels"`
	Flags       []string        `json:"Flags"`
	Repository  RepositoryInfo  `json:"Repository"`
	Host        SystemInfo      `json:"Host"`
	ProxyServer ProxyServerInfo `json:"ProxyServer"`
	Timeout     uint32          `json:"Timeout,string"`
}

// ValueEntry struct contains the result metrics for one trace replay test
type ValueEntry struct {
	Unit      string  `json:"Unit"`
	Direction int32   `json:"Direction,string"`
	Value     float32 `json:"Value,string"`
}

// TestEntryResult struct contains the result of one TestEntry
type TestEntryResult struct {
	Name    string                `json:"Name"`
	Result  string                `json:"Result"`
	Message string                `json:"Message"`
	Values  map[string]ValueEntry `json:"Values"`
}

// TestGroupResult struct is a part of guest->host communication protocol and it carries
// the test results for a whole test group
type TestGroupResult struct {
	Result  string            `json:"Result"`
	Message string            `json:"Message"`
	Entries []TestEntryResult `json:"Entries"`
}
