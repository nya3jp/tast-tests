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

type DHCPHandlingRuleInterface interface {
	String() string
	packetIsTooLate() bool
	packetIsTooSoon() bool
	handle(*DHCPPacket) int
	injectOptions(*DHCPPacket, []uint8)
	injectFields(*DHCPPacket)
	isOurMessageType(*DHCPPacket) bool

	responsePacketCount() int
	handleImpl(*DHCPPacket) int
	respond(*DHCPPacket) (*DHCPPacket, error)
}

type DHCPHandlingRule struct {
	IsFinalHandler     bool
	Options            map[optionInterface]interface{}
	Fields             map[fieldInterface]interface{}
	TargetTime         time.Time
	AllowableTimeDelta time.Duration
	ForceReplyOptions  []optionInterface
	messageType        messageType
	lastWarning        string
	ruleInterface      DHCPHandlingRuleInterface
}

func buildDHCPHandlingRule(messageType messageType, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) DHCPHandlingRule {
	rule := DHCPHandlingRule{Options: options, Fields: fields, AllowableTimeDelta: 500 * time.Millisecond, messageType: messageType}
	return rule
}

func (d *DHCPHandlingRule) setRuleInterface(ruleInterface DHCPHandlingRuleInterface) {
	d.ruleInterface = ruleInterface
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
	if d.packetIsTooSoon() {
		return responseNoAction
	}
	if d.ruleInterface == nil {
		return responseNoAction
	}
	return d.ruleInterface.handleImpl(queryPacket)
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

type RespondToDiscoveryRule struct {
	DHCPHandlingRule
	intendedIP    string
	serverIP      string
	shouldRespond bool
}

func NewRespondToDiscoveryRule(intendedIP string, serverIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool) *RespondToDiscoveryRule {
	genericRule := buildDHCPHandlingRule(messageTypeDiscovery, options, fields)
	rule := RespondToDiscoveryRule{genericRule, intendedIP, serverIP, shouldRespond}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *RespondToDiscoveryRule) responsePacketCount() int {
	return 1
}

func (d *RespondToDiscoveryRule) handleImpl(queryPacket *DHCPPacket) int {
	if !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}
	if d.shouldRespond {
		ret |= responseHaveResponse
	}
	return ret
}

func (d *RespondToDiscoveryRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if !d.isOurMessageType(queryPacket) {
		return nil, errors.New(d.lastWarning)
	}

	transactionID, err := queryPacket.transactionID()
	if err != nil {
		return nil, err
	}
	clientHWAddress, err := queryPacket.clientHWAddress()
	if err != nil {
		return nil, err
	}
	responsePacket, err := createOfferPacket(transactionID, clientHWAddress, d.intendedIP, d.serverIP)
	if err != nil {
		return nil, err
	}
	requestedParametersInterface := queryPacket.getOption(optionParameterRequestList)
	if requestedParametersInterface != nil {
		requestedParameters, ok := requestedParametersInterface.([]uint8)
		if ok {
			d.injectOptions(responsePacket, requestedParameters)
		}
	}
	d.injectFields(responsePacket)
	return responsePacket, nil
}

type RejectRequestRule struct {
	DHCPHandlingRule
	shouldRespond bool
}

func NewRejectRequestRule() *RejectRequestRule {
	genericRule := buildDHCPHandlingRule(messageTypeRequest, map[optionInterface]interface{}{}, map[fieldInterface]interface{}{})
	rule := RejectRequestRule{genericRule, true}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *RejectRequestRule) responsePacketCount() int {
	return 1
}

func (d *RejectRequestRule) handleImpl(queryPacket *DHCPPacket) int {
	if !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}
	if d.shouldRespond {
		ret |= responseHaveResponse
	}
	return ret
}

func (d *RejectRequestRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if !d.isOurMessageType(queryPacket) {
		return nil, errors.New(d.lastWarning)
	}

	transactionID, err := queryPacket.transactionID()
	if err != nil {
		return nil, err
	}
	clientHWAddress, err := queryPacket.clientHWAddress()
	if err != nil {
		return nil, err
	}
	return createNAKPacket(transactionID, clientHWAddress)
}

type RespondToRequestRule struct {
	DHCPHandlingRule
	expectedRequestedIP string
	expectedServerIP    string
	shouldRespond       bool
	grantedIP           string
	serverIP            string
	expectServerIPSet   bool
}

func NewRespondToRequestRule(expectedRequestedIP string, expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool, responseServerIP string, responseGrantedIP string, expectServerIPSet bool) *RespondToRequestRule {
	genericRule := buildDHCPHandlingRule(messageTypeRequest, options, fields)
	rule := RespondToRequestRule{genericRule, expectedRequestedIP, expectedServerIP, shouldRespond, responseGrantedIP, responseServerIP, expectServerIPSet}
	if len(rule.grantedIP) == 0 {
		rule.grantedIP = rule.expectedRequestedIP
	}
	if len(rule.serverIP) == 0 {
		rule.serverIP = rule.expectedServerIP
	}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *RespondToRequestRule) responsePacketCount() int {
	return 1
}

func (d *RespondToRequestRule) handleImpl(queryPacket *DHCPPacket) int {
	if !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	serverIP := queryPacket.getOption(optionServerID)
	requestedIP := queryPacket.getOption(optionRequestedIP)
	serverIPProvided := serverIP != nil
	if serverIPProvided != d.expectServerIPSet || requestedIP == nil {
		return responseNoAction
	}
	if serverIPProvided && serverIP != d.expectedServerIP {
		return responseNoAction
	}
	if requestedIP != d.expectedRequestedIP {
		return responseNoAction
	}

	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}
	if d.shouldRespond {
		ret |= responseHaveResponse
	}
	return ret
}

func (d *RespondToRequestRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if !d.isOurMessageType(queryPacket) {
		return nil, errors.New(d.lastWarning)
	}

	transactionID, err := queryPacket.transactionID()
	if err != nil {
		return nil, err
	}
	clientHWAddress, err := queryPacket.clientHWAddress()
	if err != nil {
		return nil, err
	}
	responsePacket, err := createAcknowledgementPacket(transactionID, clientHWAddress, d.grantedIP, d.serverIP)
	if err != nil {
		return nil, err
	}
	requestedParametersInterface := queryPacket.getOption(optionParameterRequestList)
	if requestedParametersInterface != nil {
		requestedParameters, ok := requestedParametersInterface.([]uint8)
		if ok {
			d.injectOptions(responsePacket, requestedParameters)
		}
	}
	d.injectFields(responsePacket)
	return responsePacket, nil
}

type RespondToPostT2RequestRule struct {
	RespondToRequestRule
}

func NewRespondToPostT2RequestRule(expectedRequestedIP string, responseServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool, responseGrantedIP string) *RespondToPostT2RequestRule {
	respondToRequestRule := NewRespondToRequestRule(expectedRequestedIP, "", options, fields, shouldRespond, responseServerIP, responseGrantedIP, true)
	rule := RespondToPostT2RequestRule{*respondToRequestRule}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *RespondToPostT2RequestRule) handleImpl(queryPacket *DHCPPacket) int {
	if !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	if queryPacket.getOption(optionServerID) != nil {
		return responseNoAction
	}
	requestedIP := queryPacket.getOption(optionRequestedIP)
	if requestedIP == nil {
		return responseNoAction
	}
	if requestedIP != d.expectedRequestedIP {
		return responseNoAction
	}
	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}
	if d.shouldRespond {
		ret |= responseHaveResponse
	}
	return ret
}

type AcceptReleaseRule struct {
	DHCPHandlingRule
	expectedServerIP string
}

func NewAcceptReleaseRule(expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) *AcceptReleaseRule {
	genericRule := buildDHCPHandlingRule(messageTypeRelease, options, fields)
	rule := AcceptReleaseRule{genericRule, expectedServerIP}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *AcceptReleaseRule) responsePacketCount() int {
	return 1
}

func (d *AcceptReleaseRule) handleImpl(queryPacket *DHCPPacket) int {
	if !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	serverIP := queryPacket.getOption(optionServerID)
	if serverIP == nil {
		return responseNoAction
	}
	if serverIP != d.expectedServerIP {
		return responseNoAction
	}
	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}
	return ret
}

func (d *AcceptReleaseRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	return nil, errors.New("No response for RELEASE packet")
}

type RejectAndRespondToRequestRule struct {
	RespondToRequestRule
	sendNAKBeforeAck bool
	responseCounter  int
}

func NewRejectAndRespondToRequestRule(expectedRequestedIP string, expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, sendNAKBeforeAck bool) *RejectAndRespondToRequestRule {
	respondToRequestRule := NewRespondToRequestRule(expectedRequestedIP, expectedServerIP, options, fields, true, "", "", true)
	rule := RejectAndRespondToRequestRule{*respondToRequestRule, sendNAKBeforeAck, 0}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *RejectAndRespondToRequestRule) responsePacketCount() int {
	return 2
}

func (d *RejectAndRespondToRequestRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if (d.responseCounter == 0 && d.sendNAKBeforeAck) || (d.responseCounter != 0 && !d.sendNAKBeforeAck) {
		transactionID, err := queryPacket.transactionID()
		if err != nil {
			return nil, err
		}
		clientHWAddress, err := queryPacket.clientHWAddress()
		if err != nil {
			return nil, err
		}
		responsePacket, err := createNAKPacket(transactionID, clientHWAddress)
		if err != nil {
			return nil, err
		}
		d.responseCounter++
		return responsePacket, nil
	} else {
		responsePacket, err := d.RespondToRequestRule.respond(queryPacket)
		if err != nil {
			return nil, err
		}
		d.responseCounter++
		return responsePacket, nil
	}
	return nil, errors.New("not reached")
}

type AcceptDeclineRule struct {
	DHCPHandlingRule
	expectedServerIP string
}

func NewAcceptDeclineRule(expectedServerIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) *AcceptDeclineRule {
	genericRule := buildDHCPHandlingRule(messageTypeRelease, options, fields)
	rule := AcceptDeclineRule{genericRule, expectedServerIP}
	rule.setRuleInterface(&rule)
	return &rule
}

func (d *AcceptDeclineRule) responsePacketCount() int {
	return 1
}

func (d *AcceptDeclineRule) handleImpl(queryPacket *DHCPPacket) int {
	if !d.isOurMessageType(queryPacket) {
		return responseNoAction
	}

	serverIP := queryPacket.getOption(optionServerID)
	if serverIP == nil {
		return responseNoAction
	}
	if serverIP != d.expectedServerIP {
		return responseNoAction
	}
	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}
	return ret
}

func (d *AcceptDeclineRule) respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	return nil, errors.New("No response for DECLINE packet")
}
