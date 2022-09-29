// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"fmt"
)

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

// RxPower is a predefined callbox Rx (downlink) power level in dBm.
// Note: the CallboxManager expects RxPower to be passed as a string.
type RxPower string

const (
	// LteRxPowerExcellent is a predefined excellent downlink power level.
	LteRxPowerExcellent RxPower = "-88"
	// LteRxPowerHigh is a predefined high downlink power level.
	LteRxPowerHigh = "-98"
	// LteRxPowerMedium is a predefined medium downlink power level.
	LteRxPowerMedium = "-108"
	// LteRxPowerWeak is a predefined weak downlink power level.
	LteRxPowerWeak = "-118"
	// LteRxPowerDisconnected is a predefined disconnected downlink power level.
	LteRxPowerDisconnected = "-170"
)

// NewRxPower returns a RxPower from an exact value in dBm.
func NewRxPower(power float64) RxPower {
	return RxPower(fmt.Sprintf("%f", power))
}

// ConfigureRxPowerRequestBody is the request body for ConfigureRxPower requests.
type ConfigureRxPowerRequestBody struct {
	Callbox string  `json:"callbox,omitempty"`
	Power   RxPower `json:"pdl,omitempty"`
}

// TxPower is a predefined callbox Tx (uplink) power level in dBm.
// Note: the CallboxManager expects TxPower to be passed as a string.
type TxPower string

const (
	// LteTxPowerMax is a predefined max uplink power level.
	LteTxPowerMax TxPower = "24"
	// LteTxPowerHigh is a predefined high uplink power level.
	LteTxPowerHigh = "13"
	// LteTxPowerMedium is a predefined medium uplink power level.
	LteTxPowerMedium = "3"
	// LteTxPowerLow is a predefined low uplink power level.
	LteTxPowerLow = "-20"
)

// NewTxPower returns a TxPower from an exact value in dBm.
func NewTxPower(power float64) TxPower {
	return TxPower(fmt.Sprintf("%f", power))
}

// ConfigureTxPowerRequestBody is the request body for ConfigureTxPower requests.
type ConfigureTxPowerRequestBody struct {
	Callbox string  `json:"callbox,omitempty"`
	Power   TxPower `json:"pul,omitempty"`
}

// FetchTxPowerRequestBody is the request body for FetchTxPower requests.
type FetchTxPowerRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// FetchTxPowerResponseBody is the response body for FetchTRxPower requests.
type FetchTxPowerResponseBody struct {
	Power float64 `json:"pul,omitempty"`
}

// FetchRxPowerRequestBody is the request body for FetchRxPower requests.
type FetchRxPowerRequestBody struct {
	Callbox string `json:"callbox,omitempty"`
}

// FetchRxPowerResponseBody is the response body for FetchRxPower requests.
type FetchRxPowerResponseBody struct {
	Power float64 `json:"pdl,omitempty"`
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
