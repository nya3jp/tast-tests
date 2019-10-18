// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"chromiumos/tast/errors"
)

// Response types.
const (
	// Drops the packet and acts like it never happened.
	responseNoAction = 0
	// Signals that the handler wishes to send a packet.
	responseHaveResponse = 1 << 0
	// Signals that the handler wishes to be removed from the handling queue.
	// The handler will be asked to generate a packet first if the handler
	// signaled that it wished to do so with responseHaveResponse.
	responsePopHandler = 1 << 1
	// Signals that the handler wants to end the test on a failure.
	responseTestFailed = 1 << 2
	// Signals that the handler wants to end the test because it succeeded.
	// Note that the failure bit has precedence over the success bit.
	responseTestSucceeded = 1 << 3
)

// DHCP handling rule types.
const (
	respondToDiscoveryRule        = iota
	rejectRequestRule             = iota
	respondToRequestRule          = iota
	respondToPostT2RequestRule    = iota
	acceptReleaseRule             = iota
	rejectAndRespondToRequestRule = iota
	acceptDeclineRule             = iota
)

// dhcpHandlingRule records expectations for a DhcpTestServer.
// When a handling rule reaches the front of the dhcpTestServer handling rule
// queue, the server begins to ask the rule what it should do with each incoming
// DHCP packet (in the form of a dhcpPacket). The handle() method is expected to
// return a response that indicates whether the packet should be ignored or
// responded to and whether the test failed, succeeded, or is continuing, and an
// action that refers to whether or not the rule should be be removed from the
// test server's handling rule queue.
type dhcpHandlingRule struct {
	ruleType          int
	IsFinalHandler    bool
	Options           map[optionInterface]interface{}
	Fields            map[fieldInterface]interface{}
	ForceReplyOptions []optionInterface
	msgType           msgType
	respPktCnt        int

	intendedIP    string
	svrIP         string
	shouldRespond bool

	expReqIP    string
	expSvrIP    string
	grantedIP   string
	expSvrIPSet bool

	NAKFirst    bool
	respCounter int
}

// handle is called by the test server to ask a handling rule whether it wants
// to take some action in response to a packet. The handler should return some
// combination of response* bits as described above.
func (d *dhcpHandlingRule) handle(queryPacket *dhcpPacket) int {
	if !d.isOurMsgType(queryPacket) {
		return responseNoAction
	}

	if d.ruleType == respondToRequestRule ||
		d.ruleType == respondToPostT2RequestRule ||
		d.ruleType == acceptReleaseRule ||
		d.ruleType == rejectAndRespondToRequestRule ||
		d.ruleType == acceptDeclineRule {
		svrIP := queryPacket.getOption(optionServerID)
		if (svrIP != nil) != d.expSvrIPSet || (d.expSvrIPSet && svrIP != d.expSvrIP) {
			return responseNoAction
		}
	}

	if d.ruleType == respondToRequestRule ||
		d.ruleType == respondToPostT2RequestRule ||
		d.ruleType == rejectAndRespondToRequestRule {
		if queryPacket.getOption(optionRequestedIP) != d.expReqIP {
			return responseNoAction
		}
	}

	ret := responsePopHandler
	if d.IsFinalHandler {
		ret |= responseTestSucceeded
	}

	if (d.ruleType == respondToDiscoveryRule ||
		d.ruleType == rejectRequestRule ||
		d.ruleType == respondToRequestRule ||
		d.ruleType == respondToPostT2RequestRule ||
		d.ruleType == rejectAndRespondToRequestRule) &&
		d.shouldRespond {
		ret |= responseHaveResponse
	}
	return ret
}

// respond is called by the test server to generate a packet to send back to the
// client. This method is called if and only if the response returned from
// handle() had responseHaveResponse set.
func (d *dhcpHandlingRule) respond(queryPacket *dhcpPacket) (*dhcpPacket, error) {
	if d.ruleType == acceptReleaseRule || d.ruleType == acceptDeclineRule {
		return nil, errors.Errorf("no response for packet type: %d", d.ruleType)
	}
	if !d.isOurMsgType(queryPacket) {
		return nil, errors.New("wrong message type")
	}

	// If this is a rejectAndRespondToRequest rule, we send a NAK if this is the
	// first response and |d.NAKFirst| is true, or if this is the second
	// response and |d.NAKFirst| is false.
	sendNAK := d.ruleType == rejectAndRespondToRequestRule && ((d.respCounter == 0 && d.NAKFirst) || (d.respCounter != 0 && !d.NAKFirst))

	var txnID uint32
	var clientHWAddr string
	var err error
	if d.ruleType == respondToDiscoveryRule ||
		d.ruleType == rejectRequestRule ||
		d.ruleType == respondToRequestRule ||
		d.ruleType == respondToPostT2RequestRule ||
		d.ruleType == rejectAndRespondToRequestRule {
		txnID, err = queryPacket.txnID()
		if err != nil {
			return nil, err
		}
		clientHWAddr, err = queryPacket.clientHWAddr()
		if err != nil {
			return nil, err
		}
	}

	var responsePacket *dhcpPacket
	if d.ruleType == respondToDiscoveryRule {
		responsePacket, err = createOffer(txnID, clientHWAddr, d.intendedIP, d.svrIP)
	} else if d.ruleType == rejectRequestRule || sendNAK {
		responsePacket, err = createNAK(txnID, clientHWAddr)
	} else {
		responsePacket, err = createAck(txnID, clientHWAddr, d.grantedIP, d.svrIP)
	}
	if err != nil {
		return nil, err
	}

	if d.ruleType == respondToDiscoveryRule ||
		d.ruleType == respondToRequestRule ||
		d.ruleType == respondToPostT2RequestRule ||
		(d.ruleType == rejectAndRespondToRequestRule && !sendNAK) {
		requestedParametersInterface := queryPacket.getOption(optionParameterRequestList)
		if requestedParametersInterface != nil {
			requestedParameters, ok := requestedParametersInterface.([]uint8)
			if ok {
				d.injectOptions(responsePacket, requestedParameters)
			}
		}
		d.injectFields(responsePacket)
	}
	if d.ruleType == rejectAndRespondToRequestRule {
		d.respCounter++
	}
	return responsePacket, nil
}

// injectOptions adds options listed in the intersection of
// |requestedParameters| and |d.Options| to |packet|. Also include the options
// in the intersection of |d.ForceReplyOptions| and |d.Options|.
func (d *dhcpHandlingRule) injectOptions(packet *dhcpPacket, requestedParameters []uint8) {
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

// injectFields adds fields listed in |d.fields| to |packet|.
func (d *dhcpHandlingRule) injectFields(packet *dhcpPacket) {
	for field, value := range d.Fields {
		packet.setField(field, value)
	}
}

// isOurMsgType checks if the packet's message type matches the message type
// handled by this rule.
func (d *dhcpHandlingRule) isOurMsgType(packet *dhcpPacket) bool {
	msgType, err := packet.msgType()
	if err == nil && msgType == d.msgType {
		return true
	}
	return false
}

// newRespondToDiscoveryRule creates a handler that accepts any DISCOVER packet
// received by the server. In response to such a packet, the handler will
// construct an OFFER packet offering |intendedIP| from a server at |svrIP|.
func newRespondToDiscoveryRule(intendedIP string, svrIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool) *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:      respondToDiscoveryRule,
		Options:       options,
		Fields:        fields,
		msgType:       msgTypeDiscovery,
		respPktCnt:    1,
		intendedIP:    intendedIP,
		svrIP:         svrIP,
		shouldRespond: shouldRespond,
	}
}

// newRejectRequestRule creates a handler that receives a REQUEST packet and
// responds with a NAK.
func newRejectRequestRule() *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:      rejectRequestRule,
		msgType:       msgTypeRequest,
		shouldRespond: true,
	}
}

// newRespondToRequestRule creates a handler that accepts any REQUEST packet
// that contains options for serverID and requestedIP that match
// |expSvrIP| and |expectedRequestIP| respectively. It responds with an
// ACKNOWLEDGEMENT packet from a DHCP server at |responsesvrIP| granting
// |responseGrantedIP| to a client at the address given in the REQUEST packet.
// If |responsesvrIP| or |responseGrantedIP| are not given, then they default
// to |expSvrIP| and |expReqIP| respectively.
func newRespondToRequestRule(expReqIP string, expSvrIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool, responsesvrIP string, responseGrantedIP string, expSvrIPSet bool) *dhcpHandlingRule {
	rule := dhcpHandlingRule{
		ruleType:      respondToRequestRule,
		Options:       options,
		Fields:        fields,
		msgType:       msgTypeRequest,
		respPktCnt:    1,
		expReqIP:      expReqIP,
		expSvrIP:      expSvrIP,
		shouldRespond: shouldRespond,
		grantedIP:     responseGrantedIP,
		svrIP:         responsesvrIP,
		expSvrIPSet:   expSvrIPSet,
	}
	if len(rule.grantedIP) == 0 {
		rule.grantedIP = rule.expReqIP
	}
	if len(rule.svrIP) == 0 {
		rule.svrIP = rule.expSvrIP
	}
	return &rule
}

// newRespondToPostT2RequestRule creates a handler similar to respondToRequest
// except that it expects request packets like those sent after the T2 deadline
// (see RFC 2131). This is the only time that you can find a request packet
// without the serverID option. It reseponds to packets in exactly the same way.
func newRespondToPostT2RequestRule(expReqIP string, responsesvrIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, shouldRespond bool, responseGrantedIP string) *dhcpHandlingRule {
	rule := newRespondToRequestRule(expReqIP, "", options, fields, shouldRespond, responsesvrIP, responseGrantedIP, false)
	rule.ruleType = respondToPostT2RequestRule
	return rule
}

// newAcceptReleaseRule creates a handler that accepts any RELEASE packet that
// contains an option for serverID that matches |expSvrIP|. There is no
// response to this packet.
func newAcceptReleaseRule(expSvrIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:   acceptReleaseRule,
		Options:    options,
		Fields:     fields,
		msgType:    msgTypeRelease,
		respPktCnt: 1,
		expSvrIP:   expSvrIP,
	}
}

// newRejectAndRespondToRequestRule creates a handler that accepts any REQUEST
// packet that contains options for serverID and resquestedIP that match
// |expSvrIP| and |expReqIP| respectively. It responds with
// both an ACKNOWLEDGEMENT packet from a DHCP server as well as a NAK, in order
// to simulate a network with two conflicting servers.
func newRejectAndRespondToRequestRule(expReqIP string, expSvrIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}, NAKFirst bool) *dhcpHandlingRule {
	rule := newRespondToRequestRule(expReqIP, expSvrIP, options, fields, true, "", "", true)
	rule.respPktCnt = 2
	rule.ruleType = rejectAndRespondToRequestRule
	rule.NAKFirst = NAKFirst
	return rule
}

// newAcceptDeclineRule creates a handler that accepts any DECLINE packet that
// contains an option for serverID that matches |expSvrIP|. There is no
// response to this packet.
func newAcceptDeclineRule(expSvrIP string, options map[optionInterface]interface{}, fields map[fieldInterface]interface{}) *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:   acceptDeclineRule,
		Options:    options,
		Fields:     fields,
		msgType:    msgTypeRelease,
		respPktCnt: 1,
		expSvrIP:   expSvrIP,
	}
}
