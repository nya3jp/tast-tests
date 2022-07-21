// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

// ConfigureCallboxRequestBody is the request body for ConfigureCallbox requests.
type ConfigureCallboxRequestBody struct {
	Callbox       string   `json:"callbox,omitempty"`
	Hardware      string   `json:"hardware,omitempty"`
	CellularType  string   `json:"cellular_type,omitempty"`
	ParameterList []string `json:"parameter_list,omitempty"`
}

// BeginSimulationRequestBody is the request body for BeginSimulation requests.
type BeginSimulationRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// SendSmsRequestBody is the request body for SendSms requests.
type SendSmsRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
	Message string `json:"sms,omitempty"`
}

// ConfigureIperfRequestBody is the request body for iperf configuration requests.
type ConfigureIperfRequestBody struct {
	Callbox    string              `json:"callbox,omitempty"`
	Time       int                 `json:"time,omitempty"`
	PacketSize int                 `json:"psize,omitempty"`
	Clients    []IperfClientConfig `json:"clients,omitempty"`
	Servers    []IperfServerConfig `json:"servers,omitempty"`
}

// IperfClientConfig is a configuration to use with a callbox Iperf client instance.
type IperfClientConfig struct {
	IP                  string  `json:"ip,omitempty"`
	Port                int     `json:"port,omitempty"`
	Protocol            string  `json:"proto,omitempty"`
	WindowSize          int64   `json:"wsize,omitempty"`
	ParallelConnections int     `json:"pconnections,omitempty"`
	MaxBitRate          float64 `json:"mbitrate,omitempty"`
}

// IperfServerConfig is a configuration to use with a callbox Iperf server instance.
type IperfServerConfig struct {
	IP         string `json:"ip,omitempty"`
	Port       int    `json:"port,omitempty"`
	Protocol   string `json:"proto,omitempty"`
	WindowSize int64  `json:"wsize,omitempty"`
}

// StartIperfRequestBody is the request body for iperf start requests.
type StartIperfRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// StopIperfRequestBody is the request body for Iperf stop requests.
type StopIperfRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// CloseIperfRequestBody is the request body for Iperf close requests.
type CloseIperfRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// FetchIperfResultRequestBody is the request body for Iperf results query requests.
type FetchIperfResultRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// FetchIperfResultResponseBody is the response body for an Iperf result query requests.
type FetchIperfResultResponseBody struct {
	Clients []*IperfClientResult `json:"clients"`
	Servers []*IperfServerResult `json:"servers"`
}

// IperfServerResult is the current Iperf measurement for an Iperf server instance.
type IperfServerResult struct {
	ID          int     `json:"counter"`
	Throughput  float64 `json:"throughput"`
	PercentLoss float64 `json:"loss"`
}

// IperfClientResult is the current Iperf measurement for an Iperf client instance.
type IperfClientResult struct {
	ID         int     `json:"counter"`
	Throughput float64 `json:"throughput"`
}

// FetchIperfIPRequestBody is the request body for an Iperf IP query request.
type FetchIperfIPRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// FetchIperfIPResponseBody is the response body for an Iperf IP query request.
type FetchIperfIPResponseBody struct {
	IP string `json:"ip"`
}

// FetchMaxThroughputRequestBody is the request body for a maximum throughput request.
type FetchMaxThroughputRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// FetchMaxThroughputResponseBody is the request body for a maximum throughput request.
type FetchMaxThroughputResponseBody struct {
	Uplink   float64 `json:"uplink"`
	Downlink float64 `json:"downlink"`
}
