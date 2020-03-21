// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package proto contains trace_replay application host <-> guest communication protocol structures.
package proto

const (
	// TestResultSuccess means all the tests in a test group were completed successfully
	TestResultSuccess = "Success"
	// TestResultFailure means one or several tests in the group encountered a failure
	TestResultFailure = "Failure"
)

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
	SystemBoard   string `json:"SystemBoard"`
	SystemVersion string `json:"SystemVersion"`
}

// TestGroupConfig struct is a part of host->guest communication protocol and it used
// to define a trace replay test group configuration as well as a container to deliver
// the required host environment related information inside the guest
type TestGroupConfig struct {
	Labels      []string        `json:"Labels"`
	Repository  RepositoryInfo  `json:"Repository"`
	Host        SystemInfo      `json:"Host"`
	ProxyServer ProxyServerInfo `json:"ProxyServer"`
}

// ReplayResult struct contains the result of one trace replay pass
type ReplayResult struct {
	TotalFrames       uint32  `json:"TotalFrames,string"`
	AverageFPS        float32 `json:"AverageFPS,string"`
	DurationInSeconds float32 `json:"DurationInSeconds,string"`
}

// TestEntryResult struct contains the result of one TestEntry
type TestEntryResult struct {
	Name    string         `json:"Name"`
	Result  string         `json:"Result"`
	Message string         `json:"Message"`
	Values  []ReplayResult `json:"Values"`
}

// TestGroupResult struct is a part of guest->host communication protocol and it carries
// the test results for a whole test group
type TestGroupResult struct {
	Result  string            `json:"Result"`
	Message string            `json:"Message"`
	Entries []TestEntryResult `json:"Entries"`
}

// StorageFileInfo struct contains all the necessary storage file information
type StorageFileInfo struct {
	Name      string `json:"Name"`
	Size      uint64 `json:"Size,string"`
	SHA256Sum string `json:"SHA256Sum"`
}

// TraceFileInfo struct contains all the necessary .trace file information
type TraceFileInfo struct {
	Name        string `json:"Name"`
	Size        uint64 `json:"Size,string"`
	MD5Sum      string `json:"MD5Sum"`
	Version     string `json:"Version"`
	FramesCount uint32 `json:"FramesCount,string"`
}

// TraceListEntry struct contains the detailed information about one trace file
type TraceListEntry struct {
	Name        string          `json:"Name"`
	Labels      []string        `json:"Labels"`
	Board       string          `json:"Board"`
	Time        string          `json:"Time"`
	StorageFile StorageFileInfo `json:"StorageFile"`
	TraceFile   TraceFileInfo   `json:"TraceFile"`
}

// TraceList struct contains the list of trace entries available on repository
type TraceList struct {
	Version string           `json:"Version"`
	Entries []TraceListEntry `json:"Entries"`
}
