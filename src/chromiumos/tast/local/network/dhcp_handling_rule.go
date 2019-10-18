// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"chromiumos/tast/errors"
)

// Response types.
type response int

const (
	// Drops the packet and acts like it never happened.
	noAction response = 0
	// Signals that the handler wishes to send a packet.
	haveResponse response = 1 << 0
	// Signals that the handler wishes to be removed from the handling queue.
	// The handler will be asked to generate a packet first if the handler
	// signaled that it wished to do so with responseHaveResponse.
	popHandler response = 1 << 1
	// Signals that the handler wants to end the test on a failure.
	testFailed response = 1 << 2
	// Signals that the handler wants to end the test because it succeeded.
	// Note that the failure bit has precedence over the success bit.
	testSucceeded response = 1 << 3
)

// DHCP handling rule types.
type rule int

const (
	// respondToDiscovery accepts a DISCOVER packet and responds with an OFFER.
	respondToDiscovery rule = iota
	// rejectRequest receives a REQUEST and responds with a NAK.
	rejectRequest
	// respondToRequest receives a REQUEST and responds with an ACKNOWLEDGEMENT.
	respondToRequest
	// respondToPostT2Request is similar to respondToRequest except that it
	// expects request packets like those sent after the T2 deadline.
	respondToPostT2Request
	// acceptRelease accepts any RELEASE packet.
	acceptRelease
	// rejectAndRespondToRequest accepts a REQUEST packet and responds with both
	// an ACKNOWLEDGEMENT and a NAK.
	rejectAndRespondToRequest
	// acceptDecline accepts any DECLINE packet.
	acceptDecline
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
	// ruleType describes the handling rule type.
	ruleType rule
	// isFinalHandler is true when this rule is the last handler.
	isFinalHandler bool
	// options maps the packet options to their values.
	options optionMap
	// fields maps the packet fields to their values.
	fields fieldMap
	// forceReplyOptions are options that will be injected into the response.
	forceReplyOptions []option
	// msgType is the expected message type
	msgType msgType
	// respPktCnt is the number of packets this handler will respond to.
	respPktCnt int

	// intendedIP is the IP to be offered.
	intendedIP string
	// svrIP is the server offering the intendedIP.
	svrIP string
	// shouldRespond is true if the server should respond to the DHCP packet.
	shouldRespond bool

	// expReqIP is the expected requested IP.
	expReqIP string
	// expSvrIP is the expected server IP.
	expSvrIP string
	// grantedIP is the IP to be granted.
	grantedIP string
	// expSvrIPSet is true if the serverID option is expected to be set.
	expSvrIPSet bool

	// nakFirst is true when the rejectAndRespondToRequest rule should send a NAK
	// before an ACK.
	nakFirst bool
	// respCounter is the number of responses this rule has handled.
	respCounter int
}

// handle is called by the test server to ask a handling rule whether it wants
// to take some action in response to a packet. The handler should return some
// combination of response* bits as described above.
func (d *dhcpHandlingRule) handle(queryPacket *dhcpPacket) response {
	if !d.isOurMsgType(queryPacket) {
		return noAction
	}

	if d.ruleType == respondToRequest ||
		d.ruleType == respondToPostT2Request ||
		d.ruleType == acceptRelease ||
		d.ruleType == rejectAndRespondToRequest ||
		d.ruleType == acceptDecline {
		svrIP := queryPacket.option(serverID)
		if (svrIP == nil) == d.expSvrIPSet || (d.expSvrIPSet && svrIP != d.expSvrIP) {
			return noAction
		}
	}

	if d.ruleType == respondToRequest ||
		d.ruleType == respondToPostT2Request ||
		d.ruleType == rejectAndRespondToRequest {
		if queryPacket.option(requestedIP) != d.expReqIP {
			return noAction
		}
	}

	ret := popHandler
	if d.isFinalHandler {
		ret |= testSucceeded
	}

	if (d.ruleType == respondToDiscovery ||
		d.ruleType == rejectRequest ||
		d.ruleType == respondToRequest ||
		d.ruleType == respondToPostT2Request ||
		d.ruleType == rejectAndRespondToRequest) &&
		d.shouldRespond {
		ret |= haveResponse
	}
	return ret
}

// respond is called by the test server to generate a packet to send back to the
// client. This method is called if and only if the response returned from
// handle() had haveResponse set.
func (d *dhcpHandlingRule) respond(queryPacket *dhcpPacket) (*dhcpPacket, error) {
	if d.ruleType == acceptRelease || d.ruleType == acceptDecline {
		return nil, errors.Errorf("no response for packet type: %d", d.ruleType)
	}
	if !d.isOurMsgType(queryPacket) {
		return nil, errors.New("wrong message type")
	}

	// If this is a rejectAndRespondToRequest rule, we send a NAK if this is the
	// first response and |d.nakFirst| is true, or if this is the second
	// response and |d.nakFirst| is false.
	shouldSendNAK := d.ruleType == rejectAndRespondToRequest && ((d.respCounter == 0 && d.nakFirst) || (d.respCounter != 0 && !d.nakFirst))

	var txnID uint32
	var clientHWAddr []byte
	var err error
	if d.ruleType == respondToDiscovery ||
		d.ruleType == rejectRequest ||
		d.ruleType == respondToRequest ||
		d.ruleType == respondToPostT2Request ||
		d.ruleType == rejectAndRespondToRequest {
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
	if d.ruleType == respondToDiscovery {
		responsePacket, err = createOffer(txnID, clientHWAddr, d.intendedIP, d.svrIP)
	} else if d.ruleType == rejectRequest || shouldSendNAK {
		responsePacket, err = createNAK(txnID, clientHWAddr)
	} else {
		responsePacket, err = createAck(txnID, clientHWAddr, d.grantedIP, d.svrIP)
	}
	if err != nil {
		return nil, err
	}

	if d.ruleType == respondToDiscovery ||
		d.ruleType == respondToRequest ||
		d.ruleType == respondToPostT2Request ||
		(d.ruleType == rejectAndRespondToRequest && !shouldSendNAK) {
		requestedParametersInterface := queryPacket.option(parameterRequestList)
		if requestedParametersInterface != nil {
			requestedParameters, ok := requestedParametersInterface.([]uint8)
			if ok {
				d.injectOptions(responsePacket, requestedParameters)
			}
		}
		d.injectFields(responsePacket)
	}
	if d.ruleType == rejectAndRespondToRequest {
		d.respCounter++
	}
	return responsePacket, nil
}

// injectOptions adds options listed in the intersection of
// |requestedParameters| and |d.Options| to |packet|. Also include the options
// in the intersection of |d.ForceReplyOptions| and |d.Options|.
func (d *dhcpHandlingRule) injectOptions(packet *dhcpPacket, requestedParameters []uint8) {
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

// injectFields adds fields listed in |d.fields| to |packet|.
func (d *dhcpHandlingRule) injectFields(packet *dhcpPacket) {
	for field, value := range d.fields {
		packet.setField(field, value)
	}
}

// isOurMsgType checks if the packet's message type matches the message type
// handled by this rule.
func (d *dhcpHandlingRule) isOurMsgType(packet *dhcpPacket) bool {
	msgType, err := packet.msgType()
	return err == nil && msgType == d.msgType
}

// newRespondToDiscovery creates a handler that accepts any DISCOVER packet
// received by the server. In response to such a packet, the handler will
// construct an OFFER packet offering |intendedIP| from a server at |svrIP|.
func newRespondToDiscovery(intendedIP string, svrIP string, options optionMap, fields fieldMap, shouldRespond bool) *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:      respondToDiscovery,
		options:       options,
		fields:        fields,
		msgType:       discovery,
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
		ruleType:      rejectRequest,
		msgType:       request,
		shouldRespond: true,
	}
}

// newRespondToRequest creates a handler that accepts any REQUEST packet that
// contains options for serverID and requestedIP that match |expSvrIP| and
// |expectedRequestIP| respectively. It responds with an ACKNOWLEDGEMENT packet
// from a DHCP server at |responsesvrIP| granting |responseGrantedIP| to a
// client at the address given in the REQUEST packet. If |responsesvrIP| or
// |responseGrantedIP| are not given, then they default to |expSvrIP| and
// |expReqIP| respectively.
func newRespondToRequest(expReqIP string, expSvrIP string, options optionMap, fields fieldMap, shouldRespond bool, responsesvrIP string, responseGrantedIP string, expSvrIPSet bool) *dhcpHandlingRule {
	rule := dhcpHandlingRule{
		ruleType:      respondToRequest,
		options:       options,
		fields:        fields,
		msgType:       request,
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

// newRespondToPostT2Request creates a handler similar to respondToRequest
// except that it expects request packets like those sent after the T2 deadline
// (see RFC 2131). This is the only time that you can find a request packet
// without the serverID option. It reseponds to packets in exactly the same way.
func newRespondToPostT2Request(expReqIP string, responseSvrIP string, options optionMap, fields fieldMap, shouldRespond bool, responseGrantedIP string) *dhcpHandlingRule {
	rule := newRespondToRequest(expReqIP, "", options, fields, shouldRespond, responseSvrIP, responseGrantedIP, false)
	rule.ruleType = respondToPostT2Request
	return rule
}

// newAcceptRelease creates a handler that accepts any RELEASE packet that
// contains an option for serverID that matches |expSvrIP|. There is no
// response to this packet.
func newAcceptRelease(expSvrIP string, options optionMap, fields fieldMap) *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:   acceptRelease,
		options:    options,
		fields:     fields,
		msgType:    release,
		respPktCnt: 1,
		expSvrIP:   expSvrIP,
	}
}

// newRejectAndRespondToRequest creates a handler that accepts any REQUEST
// packet that contains options for serverID and resquestedIP that match
// |expSvrIP| and |expReqIP| respectively. It responds with
// both an ACKNOWLEDGEMENT packet from a DHCP server as well as a NAK, in order
// to simulate a network with two conflicting servers.
func newRejectAndRespondToRequest(expReqIP string, expSvrIP string, options optionMap, fields fieldMap, nakFirst bool) *dhcpHandlingRule {
	rule := newRespondToRequest(expReqIP, expSvrIP, options, fields, true, "", "", true)
	rule.respPktCnt = 2
	rule.ruleType = rejectAndRespondToRequest
	rule.nakFirst = nakFirst
	return rule
}

// newAcceptDecline creates a handler that accepts any DECLINE packet that
// contains an option for serverID that matches |expSvrIP|. There is no
// response to this packet.
func newAcceptDecline(expSvrIP string, options optionMap, fields fieldMap) *dhcpHandlingRule {
	return &dhcpHandlingRule{
		ruleType:   acceptDecline,
		options:    options,
		fields:     fields,
		msgType:    release,
		respPktCnt: 1,
		expSvrIP:   expSvrIP,
	}
}
