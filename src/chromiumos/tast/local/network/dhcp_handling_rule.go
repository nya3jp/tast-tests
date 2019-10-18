// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"fmt"
	"time"

	"chromiumos/tast/errors"
)

const (
	// Drops the packet and acts like it never happened.
	responseNoAction = 0
	// Signals that the handler wishes to send a packet.
	responseHaveResponse = 1 << 0
	// Signals that the handler wishes to be removed from the handling queue.
	// The handler will be asked to generate a packet first if the handler signaled
	// that it wished to do so with responseHaveResponse.
	responsePopHandler = 1 << 1
	// Signals that the handler wants to end the test on a failure.
	responseTestFailed = 1 << 2
	// Signals that the handler wants to end the test because it succeeded.
	// Note that the failure bit has precedence over the success bit.
	responseTestSucceeded = 1 << 3
)

const (
	RespondToDiscoveryRule        = iota
	RejectRequestRule             = iota
	RespondToRequestRule          = iota
	RespondToPostT2RequestRule    = iota
	AcceptReleaseRule             = iota
	RejectAndRespondToRequestRule = iota
	AcceptDeclineRule             = iota
)

type DHCPHandlingRule struct {
	ruleType            int
	IsFinalHandler      bool
	Options             map[optionInterface]interface{}
	Fields              map[fieldInterface]interface{}
	TargetTime          time.Time
	AllowableTimeDelta  time.Duration
	ForceReplyOptions   []optionInterface
	messageType         messageType
	lastWarning         string
	responsePacketCount int

	intendedIP    string
	serverIP      string
	shouldRespond bool

	expectedRequestedIP string
	expectedServerIP    string
	grantedIP           string
	expectServerIPSet   bool

	sendNAKBeforeAck bool
	responseCounter  int
}

func (d *DHCPHandlingRule) String() string {
	return fmt.Sprintf("%T", d)
}

func (d *DHCPHandlingRule) packetIsTooLate() bool {
	if d.TargetTime.IsZero() {
		return false
	}
	delta := time.Now().Sub(d.TargetTime)
	if delta > d.AllowableTimeDelta {
		return true
	}
	return false
}

func (d *DHCPHandlingRule) packetIsTooSoon() bool {
	if d.TargetTime.IsZero() {
		return false
	}
	delta := d.TargetTime.Sub(time.Now())
	if delta > d.AllowableTimeDelta {
		return true
	}
	return false
}

func (d *DHCPHandlingRule) handle(queryPacket *DHCPPacket) int {
	if d.packetIsTooLate() {
		return responseHaveResponse
	}
	if d.packetIsTooSoon() || !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	if d.ruleType == RespondToRequestRule ||
		d.ruleType == RespondToPostT2RequestRule ||
		d.ruleType == AcceptReleaseRule ||
		d.ruleType == RejectAndRespondToRequestRule ||
		d.ruleType == AcceptDeclineRule {
		serverIP := queryPacket.getOption(optionServerID)
		if (serverIP != nil) != d.expectServerIPSet || (d.expectServerIPSet && serverIP != d.expectedServerIP) {
			return responseNoAction
		}
	}

	if d.ruleType == RespondToRequestRule ||
		d.ruleType == RespondToPostT2RequestRule ||
		d.ruleType == RejectAndRespondToRequestRule {
		if queryPacket.getOption(optionRequestedIP) != d.expectedRequestedIP {
			return responseNoAction
		}
	}

	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}

	if (d.ruleType == RespondToDiscoveryRule ||
		d.ruleType == RejectRequestRule ||
		d.ruleType == RespondToRequestRule ||
		d.ruleType == RespondToPostT2RequestRule ||
		d.ruleType == RejectAndRespondToRequestRule) &&
		d.shouldRespond {
		ret |= responseHaveResponse
	}
	return ret
}

func (d *DHCPHandlingRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if d.ruleType == AcceptReleaseRule || d.ruleType == AcceptDeclineRule {
		return nil, errors.Errorf("no response for packet type: %d", d.ruleType)
	}
	if !d.isOurMessageType(queryPacket) {
		return nil, errors.New("wrong message type")
	}
	sendNAK := d.ruleType == RejectAndRespondToRequestRule && ((d.responseCounter == 0 && d.sendNAKBeforeAck) || (d.responseCounter != 0 && !d.sendNAKBeforeAck))

	var transactionID uint32
	var clientHWAddress string
	var err error
	if d.ruleType == RespondToDiscoveryRule ||
		d.ruleType == RejectRequestRule ||
		d.ruleType == RespondToRequestRule ||
		d.ruleType == RespondToPostT2RequestRule ||
		d.ruleType == RejectAndRespondToRequestRule {
		transactionID, err = queryPacket.transactionID()
		if err != nil {
			return nil, err
		}
		clientHWAddress, err = queryPacket.clientHWAddress()
		if err != nil {
			return nil, err
		}
	}

	var responsePacket *DHCPPacket
	if d.ruleType == RespondToDiscoveryRule {
		responsePacket, err = createOfferPacket(transactionID, clientHWAddress, d.intendedIP, d.serverIP)
	} else if d.ruleType == RejectRequestRule || sendNAK {
		responsePacket, err = createNAKPacket(transactionID, clientHWAddress)
	} else {
		responsePacket, err = createAcknowledgementPacket(transactionID, clientHWAddress, d.grantedIP, d.serverIP)
	}
	if err != nil {
		return nil, err
	}

	if d.ruleType == RespondToDiscoveryRule ||
		d.ruleType == RespondToRequestRule ||
		d.ruleType == RespondToPostT2RequestRule ||
		(d.ruleType == RejectAndRespondToRequestRule && !sendNAK) {
		requestedParametersInterface := queryPacket.getOption(optionParameterRequestList)
		if requestedParametersInterface != nil {
			requestedParameters, ok := requestedParametersInterface.([]uint8)
			if ok {
				d.injectOptions(responsePacket, requestedParameters)
			}
		}
		d.injectFields(responsePacket)
	}
	if d.ruleType == RejectAndRespondToRequestRule {
		d.responseCounter++
	}
	return responsePacket, nil
}

func (d *DHCPHandlingRule) injectOptions(packet *DHCPPacket, requestedParameters []uint8) {
	for option, value := range d.Options {
		shouldSet := false
		for _, param := range requestedParameters {
			if option.number() == param {
				shouldSet = true
				break
			}
		}
		if !shouldSet {
			for _, replyOption := range d.ForceReplyOptions {
				if option == replyOption {
					shouldSet = true
					break
				}
			}
		}
		if shouldSet {
			packet.setOption(option, value)
		}
	}
}

func (d *DHCPHandlingRule) injectFields(packet *DHCPPacket) {
	for field, value := range d.Fields {
		packet.setField(field, value)
	}
}

func (d *DHCPHandlingRule) isOurMessageType(packet *DHCPPacket) bool {
	messageType, err := packet.messageType()
	if err == nil && messageType == d.messageType {
		return true
	}
	return false
}

func NewRespondToDiscoveryRule(intendedIP string, serverIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool) *DHCPHandlingRule {
	return &DHCPHandlingRule{
		ruleType:            RespondToDiscoveryRule,
		Options:             options,
		Fields:              fields,
		AllowableTimeDelta:  500 * time.Millisecond,
		messageType:         messageTypeDiscovery,
		responsePacketCount: 1,
		intendedIP:          intendedIP,
		serverIP:            serverIP,
		shouldRespond:       shouldRespond,
	}
}

func NewRejectRequestRule() *DHCPHandlingRule {
	return &DHCPHandlingRule{
		ruleType:            RejectRequestRule,
		AllowableTimeDelta:  500 * time.Millisecond,
		messageType:         messageTypeRequest,
		responsePacketCount: 1,
		shouldRespond:       true,
	}
}

func NewRespondToRequestRule(expectedRequestedIP string, expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool, responseServerIP string, responseGrantedIP string, expectServerIPSet bool) *DHCPHandlingRule {
	rule := DHCPHandlingRule{
		ruleType:            RespondToRequestRule,
		Options:             options,
		Fields:              fields,
		AllowableTimeDelta:  500 * time.Millisecond,
		messageType:         messageTypeRequest,
		responsePacketCount: 1,
		expectedRequestedIP: expectedRequestedIP,
		expectedServerIP:    expectedServerIP,
		shouldRespond:       shouldRespond,
		grantedIP:           responseGrantedIP,
		serverIP:            responseServerIP,
		expectServerIPSet:   expectServerIPSet,
	}
	if len(rule.grantedIP) == 0 {
		rule.grantedIP = rule.expectedRequestedIP
	}
	if len(rule.serverIP) == 0 {
		rule.serverIP = rule.expectedServerIP
	}
	return &rule
}

func NewRespondToPostT2RequestRule(expectedRequestedIP string, responseServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool, responseGrantedIP string) *DHCPHandlingRule {
	rule := NewRespondToRequestRule(expectedRequestedIP, "", options, fields, shouldRespond, responseServerIP, responseGrantedIP, false)
	rule.ruleType = RespondToPostT2RequestRule
	return rule
}

func NewAcceptReleaseRule(expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) *DHCPHandlingRule {
	return &DHCPHandlingRule{
		ruleType:            AcceptReleaseRule,
		Options:             options,
		Fields:              fields,
		AllowableTimeDelta:  500 * time.Millisecond,
		messageType:         messageTypeRelease,
		responsePacketCount: 1,
		expectedServerIP:    expectedServerIP,
	}
}

func NewRejectAndRespondToRequestRule(expectedRequestedIP string, expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, sendNAKBeforeAck bool) *DHCPHandlingRule {
	rule := NewRespondToRequestRule(expectedRequestedIP, expectedServerIP, options, fields, true, "", "", true)
	rule.responsePacketCount = 2
	rule.ruleType = RejectAndRespondToRequestRule
	rule.sendNAKBeforeAck = sendNAKBeforeAck
	return rule
}

func NewAcceptDeclineRule(expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) *DHCPHandlingRule {
	return &DHCPHandlingRule{
		ruleType:            AcceptDeclineRule,
		Options:             options,
		Fields:              fields,
		AllowableTimeDelta:  500 * time.Millisecond,
		messageType:         messageTypeRelease,
		responsePacketCount: 1,
		expectedServerIP:    expectedServerIP,
	}
}
