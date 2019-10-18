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
	RESPONSE_NO_ACTION = 0
	// Signals that the handler wishes to send a packet.
	RESPONSE_HAVE_RESPONSE = 1 << 0
	// Signals that the handler wishes to be removed from the handling queue.
	// The handler will be asked to generate a packet first if the handler signalled
	// that it wished to do so with RESPONSE_HAVE_RESPONSE.
	RESPONSE_POP_HANDLER = 1 << 1
	// Signals that the handler wants to end the test on a failure.
	RESPONSE_TEST_FAILED = 1 << 2
	// Signals that the handler wants to end the test because it succeeded.
	// Note that the failure bit has precedence over the success bit.
	RESPONSE_TEST_SUCCEEDED = 1 << 3
)

type DHCPHandlingRuleInterface interface {
	String() string
	IsFinalHandler() bool
	SetIsFinalHandler(value bool)
	GetOptions() map[OptionInterface]interface{}
	GetFields() map[FieldInterface]interface{}
	TargetTime() time.Time
	SetTargetTime(time.Time)
	GetAllowableTimeDelta() time.Duration
	SetAllowableTimeDelta(time.Duration)
	PacketIsTooLate() bool
	PacketIsTooSoon() bool
	GetForceReplyOptions() []OptionInterface
	SetForceReplyOptions([]OptionInterface)
	EmitWarning(string)
	Handle(*DHCPPacket) int
	InjectOptions(*DHCPPacket, []uint8)
	InjectFields(*DHCPPacket)
	IsOurMessageType(*DHCPPacket) bool

	ResponsePacketCount() int
	HandleImpl(*DHCPPacket) int
	Respond(*DHCPPacket) (*DHCPPacket, error)
}

type DHCPHandlingRule struct {
	isFinalHandler bool
	// logger
	options            map[OptionInterface]interface{}
	fields             map[FieldInterface]interface{}
	targetTime         time.Time
	allowableTimeDelta time.Duration
	forceReplyOptions  []OptionInterface
	messageType        MessageType
	lastWarning        string
	ruleInterface      DHCPHandlingRuleInterface
}

func BuildDHCPHandlingRule(messageType MessageType, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}) DHCPHandlingRule {
	rule := DHCPHandlingRule{options: options, fields: fields, allowableTimeDelta: 500 * time.Millisecond, messageType: messageType}
	return rule
}

func (d *DHCPHandlingRule) SetRuleInterface(ruleInterface DHCPHandlingRuleInterface) {
	d.ruleInterface = ruleInterface
}

func (d *DHCPHandlingRule) String() string {
	if len(d.lastWarning) > 0 {
		return fmt.Sprintf("%T (%s)", d, d.lastWarning)
	}
	return fmt.Sprintf("%T", d)
}

// func (d *DHCPHandlingRule) logger()

func (d *DHCPHandlingRule) IsFinalHandler() bool {
	return d.isFinalHandler
}

func (d *DHCPHandlingRule) SetIsFinalHandler(value bool) {
	d.isFinalHandler = value
}

func (d *DHCPHandlingRule) GetOptions() map[OptionInterface]interface{} {
	return d.options
}

func (d *DHCPHandlingRule) GetFields() map[FieldInterface]interface{} {
	return d.fields
}

func (d *DHCPHandlingRule) TargetTime() time.Time {
	return d.targetTime
}

func (d *DHCPHandlingRule) SetTargetTime(value time.Time) {
	d.targetTime = value
}

func (d *DHCPHandlingRule) GetAllowableTimeDelta() time.Duration {
	return d.allowableTimeDelta
}

func (d *DHCPHandlingRule) SetAllowableTimeDelta(value time.Duration) {
	d.allowableTimeDelta = value
}

func (d *DHCPHandlingRule) PacketIsTooLate() bool {
	if d.targetTime.IsZero() {
		return false
	}
	delta := time.Now().Sub(d.targetTime)
	if delta > d.allowableTimeDelta {
		return true
	}
	return false
}

func (d *DHCPHandlingRule) PacketIsTooSoon() bool {
	if d.targetTime.IsZero() {
		return false
	}
	delta := d.targetTime.Sub(time.Now())
	if delta > d.allowableTimeDelta {
		return true
	}
	return false
}

func (d *DHCPHandlingRule) GetForceReplyOptions() []OptionInterface {
	return d.forceReplyOptions
}

func (d *DHCPHandlingRule) SetForceReplyOptions(forceReplyOptions []OptionInterface) {
	d.forceReplyOptions = forceReplyOptions
}

func (d *DHCPHandlingRule) EmitWarning(warning string) {
	d.lastWarning = warning
}

func (d *DHCPHandlingRule) Handle(queryPacket *DHCPPacket) int {
	if d.PacketIsTooLate() {
		return RESPONSE_HAVE_RESPONSE
	}
	if d.PacketIsTooSoon() {
		return RESPONSE_NO_ACTION
	}
	if d.ruleInterface == nil {
		return RESPONSE_NO_ACTION
	}
	return d.ruleInterface.HandleImpl(queryPacket)
}

func (d *DHCPHandlingRule) InjectOptions(packet *DHCPPacket, requestedParameters []uint8) {
	for option, value := range d.options {
		shouldSet := false
		for _, param := range requestedParameters {
			if option.number() == param {
				shouldSet = true
				break
			}
		}
		if !shouldSet {
			for _, replyOption := range d.forceReplyOptions {
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

func (d *DHCPHandlingRule) InjectFields(packet *DHCPPacket) {
	for field, value := range d.fields {
		packet.setField(field, value)
	}
}

func (d *DHCPHandlingRule) IsOurMessageType(packet *DHCPPacket) bool {
	messageType, err := packet.messageType()
	if err == nil && messageType == d.messageType {
		return true
	}
	d.EmitWarning(fmt.Sprintf("Packet's message type was %s, not %s.", messageType.name, d.messageType.name))
	return false
}

type RespondToDiscoveryRule struct {
	DHCPHandlingRule
	intendedIP    string
	serverIP      string
	shouldRespond bool
}

func NewRespondToDiscoveryRule(intendedIP string, serverIP string, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}, shouldRespond bool) *RespondToDiscoveryRule {
	genericRule := BuildDHCPHandlingRule(MESSAGE_TYPE_DISCOVERY, options, fields)
	rule := RespondToDiscoveryRule{genericRule, intendedIP, serverIP, shouldRespond}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *RespondToDiscoveryRule) ResponsePacketCount() int {
	return 1
}

func (d *RespondToDiscoveryRule) HandleImpl(queryPacket *DHCPPacket) int {
	if !d.IsOurMessageType(queryPacket) {
		return RESPONSE_NO_ACTION
	}

	ret := RESPONSE_POP_HANDLER
	if d.IsFinalHandler() {
		ret |= RESPONSE_TEST_SUCCEEDED
	}
	if d.shouldRespond {
		ret |= RESPONSE_HAVE_RESPONSE
	}
	return ret
}

func (d *RespondToDiscoveryRule) Respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if !d.IsOurMessageType(queryPacket) {
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
	responsePacket, err := CreateOfferPacket(transactionID, clientHWAddress, d.intendedIP, d.serverIP)
	if err != nil {
		return nil, err
	}
	requestedParametersInterface := queryPacket.getOption(OPTION_PARAMETER_REQUEST_LIST)
	if requestedParametersInterface != nil {
		requestedParameters, ok := requestedParametersInterface.([]uint8)
		if ok {
			d.InjectOptions(responsePacket, requestedParameters)
		}
	}
	d.InjectFields(responsePacket)
	return responsePacket, nil
}

type RejectRequestRule struct {
	DHCPHandlingRule
	shouldRespond bool
}

func NewRejectRequestRule() *RejectRequestRule {
	genericRule := BuildDHCPHandlingRule(MESSAGE_TYPE_REQUEST, map[OptionInterface]interface{}{}, map[FieldInterface]interface{}{})
	rule := RejectRequestRule{genericRule, true}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *RejectRequestRule) ResponsePacketCount() int {
	return 1
}

func (d *RejectRequestRule) HandleImpl(queryPacket *DHCPPacket) int {
	if !d.IsOurMessageType(queryPacket) {
		return RESPONSE_NO_ACTION
	}

	ret := RESPONSE_POP_HANDLER
	if d.IsFinalHandler() {
		ret |= RESPONSE_TEST_SUCCEEDED
	}
	if d.shouldRespond {
		ret |= RESPONSE_HAVE_RESPONSE
	}
	return ret
}

func (d *RejectRequestRule) Respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if !d.IsOurMessageType(queryPacket) {
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
	return CreateNAKPacket(transactionID, clientHWAddress)
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

func NewRespondToRequestRule(expectedRequestedIP string, expectedServerIP string, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}, shouldRespond bool, responseServerIP string, responseGrantedIP string, expectServerIPSet bool) *RespondToRequestRule {
	genericRule := BuildDHCPHandlingRule(MESSAGE_TYPE_REQUEST, options, fields)
	rule := RespondToRequestRule{genericRule, expectedRequestedIP, expectedServerIP, shouldRespond, responseGrantedIP, responseServerIP, expectServerIPSet}
	if len(rule.grantedIP) == 0 {
		rule.grantedIP = rule.expectedRequestedIP
	}
	if len(rule.serverIP) == 0 {
		rule.serverIP = rule.expectedServerIP
	}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *RespondToRequestRule) ResponsePacketCount() int {
	return 1
}

func (d *RespondToRequestRule) HandleImpl(queryPacket *DHCPPacket) int {
	if !d.IsOurMessageType(queryPacket) {
		return RESPONSE_NO_ACTION
	}

	serverIP := queryPacket.getOption(OPTION_SERVER_ID)
	requestedIP := queryPacket.getOption(OPTION_REQUESTED_IP)
	serverIPProvided := serverIP != nil
	if serverIPProvided != d.expectServerIPSet || requestedIP == nil {
		return RESPONSE_NO_ACTION
	}
	if serverIPProvided && serverIP != d.expectedServerIP {
		d.EmitWarning(fmt.Sprintf("REQUEST packet's server IP did not match our expectations; expected %v but got %v", d.expectedServerIP, serverIP))
		return RESPONSE_NO_ACTION
	}
	if requestedIP != d.expectedRequestedIP {
		d.EmitWarning(fmt.Sprintf("REQUEST packet's requested IP did not match our expectations; expected %v but got %v", d.expectedRequestedIP, requestedIP))
		return RESPONSE_NO_ACTION
	}

	ret := RESPONSE_POP_HANDLER
	if d.IsFinalHandler() {
		ret |= RESPONSE_TEST_SUCCEEDED
	}
	if d.shouldRespond {
		ret |= RESPONSE_HAVE_RESPONSE
	}
	return ret
}

func (d *RespondToRequestRule) Respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if !d.IsOurMessageType(queryPacket) {
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
	responsePacket, err := CreateAcknowledgementPacket(transactionID, clientHWAddress, d.grantedIP, d.serverIP)
	if err != nil {
		return nil, err
	}
	requestedParametersInterface := queryPacket.getOption(OPTION_PARAMETER_REQUEST_LIST)
	if requestedParametersInterface != nil {
		requestedParameters, ok := requestedParametersInterface.([]uint8)
		if ok {
			d.InjectOptions(responsePacket, requestedParameters)
		}
	}
	d.InjectFields(responsePacket)
	return responsePacket, nil
}

type RespondToPostT2RequestRule struct {
	RespondToRequestRule
}

func NewRespondToPostT2RequestRule(expectedRequestedIP string, responseServerIP string, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}, shouldRespond bool, responseGrantedIP string) *RespondToPostT2RequestRule {
	respondToRequestRule := NewRespondToRequestRule(expectedRequestedIP, "", options, fields, shouldRespond, responseServerIP, responseGrantedIP, true)
	rule := RespondToPostT2RequestRule{*respondToRequestRule}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *RespondToPostT2RequestRule) HandleImpl(queryPacket *DHCPPacket) int {
	if !d.IsOurMessageType(queryPacket) {
		return RESPONSE_NO_ACTION
	}

	if queryPacket.getOption(OPTION_SERVER_ID) != nil {
		return RESPONSE_NO_ACTION
	}
	requestedIP := queryPacket.getOption(OPTION_REQUESTED_IP)
	if requestedIP == nil {
		return RESPONSE_NO_ACTION
	}
	if requestedIP != d.expectedRequestedIP {
		d.EmitWarning(fmt.Sprintf("REQUEST packet's requested IP did not match our expectations; expeceted %v but got %v", d.expectedRequestedIP, requestedIP))
		return RESPONSE_NO_ACTION
	}
	ret := RESPONSE_POP_HANDLER
	if d.IsFinalHandler() {
		ret |= RESPONSE_TEST_SUCCEEDED
	}
	if d.shouldRespond {
		ret |= RESPONSE_HAVE_RESPONSE
	}
	return ret
}

type AcceptReleaseRule struct {
	DHCPHandlingRule
	expectedServerIP string
}

func NewAcceptReleaseRule(expectedServerIP string, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}) *AcceptReleaseRule {
	genericRule := BuildDHCPHandlingRule(MESSAGE_TYPE_RELEASE, options, fields)
	rule := AcceptReleaseRule{genericRule, expectedServerIP}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *AcceptReleaseRule) ResponsePacketCount() int {
	return 1
}

func (d *AcceptReleaseRule) HandleImpl(queryPacket *DHCPPacket) int {
	if !d.IsOurMessageType(queryPacket) {
		return RESPONSE_NO_ACTION
	}

	serverIP := queryPacket.getOption(OPTION_SERVER_ID)
	if serverIP == nil {
		return RESPONSE_NO_ACTION
	}
	if serverIP != d.expectedServerIP {
		d.EmitWarning(fmt.Sprintf("RELEASE packet's server IP did not match our expectations; expected %v but got %v", d.expectedServerIP, serverIP))
		return RESPONSE_NO_ACTION
	}
	ret := RESPONSE_POP_HANDLER
	if d.IsFinalHandler() {
		ret |= RESPONSE_TEST_SUCCEEDED
	}
	return ret
}

func (d *AcceptReleaseRule) Respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	return nil, errors.New("No response for RELEASE packet")
}

type RejectAndRespondToRequestRule struct {
	RespondToRequestRule
	sendNAKBeforeAck bool
	responseCounter  int
}

func NewRejectAndRespondToRequestRule(expectedRequestedIP string, expectedServerIP string, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}, sendNAKBeforeAck bool) *RejectAndRespondToRequestRule {
	respondToRequestRule := NewRespondToRequestRule(expectedRequestedIP, expectedServerIP, options, fields, true, "", "", true)
	rule := RejectAndRespondToRequestRule{*respondToRequestRule, sendNAKBeforeAck, 0}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *RejectAndRespondToRequestRule) ResponsePacketCount() int {
	return 2
}

func (d *RejectAndRespondToRequestRule) Respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	if (d.responseCounter == 0 && d.sendNAKBeforeAck) || (d.responseCounter != 0 && !d.sendNAKBeforeAck) {
		transactionID, err := queryPacket.transactionID()
		if err != nil {
			return nil, err
		}
		clientHWAddress, err := queryPacket.clientHWAddress()
		if err != nil {
			return nil, err
		}
		responsePacket, err := CreateNAKPacket(transactionID, clientHWAddress)
		if err != nil {
			return nil, err
		}
		d.responseCounter++
		return responsePacket, nil
	} else {
		responsePacket, err := d.RespondToRequestRule.Respond(queryPacket)
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

func NewAcceptDeclineRule(expectedServerIP string, options map[OptionInterface]interface{}, fields map[FieldInterface]interface{}) *AcceptDeclineRule {
	genericRule := BuildDHCPHandlingRule(MESSAGE_TYPE_RELEASE, options, fields)
	rule := AcceptDeclineRule{genericRule, expectedServerIP}
	rule.SetRuleInterface(&rule)
	return &rule
}

func (d *AcceptDeclineRule) ResponsePacketCount() int {
	return 1
}

func (d *AcceptDeclineRule) HandleImpl(queryPacket *DHCPPacket) int {
	if !d.IsOurMessageType(queryPacket) {
		return RESPONSE_NO_ACTION
	}

	serverIP := queryPacket.getOption(OPTION_SERVER_ID)
	if serverIP == nil {
		return RESPONSE_NO_ACTION
	}
	if serverIP != d.expectedServerIP {
		d.EmitWarning(fmt.Sprintf("DECLINE packet's server IP did not match our expectations; expected %v but got %v", d.expectedServerIP, serverIP))
		return RESPONSE_NO_ACTION
	}
	ret := RESPONSE_POP_HANDLER
	if d.IsFinalHandler() {
		ret |= RESPONSE_TEST_SUCCEEDED
	}
	return ret
}

func (d *AcceptDeclineRule) Respond(queryPacket *DHCPPacket) (*DHCPPacket, error) {
	return nil, errors.New("No response for DECLINE packet")
}
