// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"

	"chromiumos/tast/errors"
)

type optionInterface interface {
	pack(interface{}) ([]byte, error)
	unpack([]byte) (interface{}, error)
	name() string
	number() uint8
}

type option struct {
	nameField   string
	numberField uint8
}

func (o option) name() string {
	return o.nameField
}

func (o option) number() uint8 {
	return o.numberField
}

type byteOption struct {
	option
}

func (o byteOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return nil, errors.New("expected uint8")
	}
	return []byte{valInt}, nil
}

func (o byteOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(bytes[0]), nil
}

type shortOption struct {
	option
}

func (o shortOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint16)
	if !ok {
		return nil, errors.New("expected uint16")
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, valInt)
	return buf, nil
}

func (o shortOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16(bytes), nil
}

type intOption struct {
	option
}

func (o intOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint32)
	if !ok {
		return nil, errors.New("expected uint32")
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, valInt)
	return buf, nil
}

func (o intOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32(bytes), nil
}

type IPAddressOption struct {
	option
}

func IPToBytes(IPAddr string) ([]byte, error) {
	IP := net.ParseIP(IPAddr)
	if IP == nil {
		return nil, errors.Errorf("unable to parse IP: %s", IPAddr)
	}
	return IP.To4(), nil
}

func bytesToIP(buf []byte) (string, error) {
	byteStr := string(buf)
	if len(buf) > 4 {
		return "", errors.New("byte string is too long")
	} else {
		byteStr += strings.Repeat("\x00", 4-len(buf))
	}
	IP := net.IP(byteStr)
	return IP.String(), nil
}

func (o IPAddressOption) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return IPToBytes(valStr)
}

func (o IPAddressOption) unpack(bytes []byte) (interface{}, error) {
	return bytesToIP(bytes)
}

type IPListOption struct {
	option
}

func (o IPListOption) pack(value interface{}) ([]byte, error) {
	valSlice, ok := value.([]string)
	if !ok {
		return nil, errors.New("expected string slice")
	}
	var bytes []byte
	for _, addr := range valSlice {
		IPBytes, err := IPToBytes(addr)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, IPBytes...)
	}
	return bytes, nil
}

func (o IPListOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes)%4 != 0 {
		return nil, errors.New("unable to parse list")
	}
	var IPList []string
	for i := 0; i < len(bytes); i += 4 {
		IPString, err := bytesToIP(bytes[i : i+4])
		if err != nil {
			return nil, err
		}
		IPList = append(IPList, IPString)
	}
	return IPList, nil
}

type rawOption struct {
	option
}

func (o rawOption) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice")
	}
	return valStr, nil
}

func (o rawOption) unpack(bytes []byte) (interface{}, error) {
	return bytes, nil
}

type byteListOption struct {
	option
}

func (o byteListOption) pack(value interface{}) ([]byte, error) {
	valBytes, ok := value.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice")
	}
	return valBytes, nil
}

func (o byteListOption) unpack(bytes []byte) (interface{}, error) {
	return bytes, nil
}

type classlessStaticRoutesOption struct {
	option
}

type staticRoute struct {
	prefixSize uint8
	destinationAddress string
	routerAddress string
}

func (o classlessStaticRoutesOption) pack(value interface{}) ([]byte, error) {
	routeList, ok := value.([]staticRoute)
	if !ok {
		return nil, errors.New("expected staticRoute slice")
	}
	var byteStr string
	for _, route := range routeList {
		byteStr += string([]byte{route.prefixSize})
		destinationAddressCount := (route.prefixSize + 7) / 8
		destinationAddressBytes, err := IPToBytes(route.destinationAddress)
		if err != nil {
			return nil, err
		}
		byteStr += string(destinationAddressBytes)[:destinationAddressCount]
		routerAddressBytes, err := IPToBytes(route.routerAddress)
		if err != nil {
			return nil, err
		}
		byteStr += string(routerAddressBytes)
	}
	return []byte(byteStr), nil
}

func (o classlessStaticRoutesOption) unpack(bytes []byte) (interface{}, error) {
	var routeList []staticRoute
	offset := 0
	for offset < len(bytes) {
		prefixSize := int(bytes[offset])
		destinationAddressCount := (prefixSize + 7) / 8
		entryEnd := offset + 1 + destinationAddressCount + 4
		if entryEnd > len(bytes) {
			return nil, errors.New("classless domain list is corrupted")
		}
		offset++
		destinationAddressEnd := offset + destinationAddressCount
		destinationAddress, err := bytesToIP(bytes[offset:destinationAddressEnd])
		if err != nil {
			return nil, err
		}
		routerAddress, err := bytesToIP(bytes[destinationAddressEnd:entryEnd])
		if err != nil {
			return nil, err
		}
		routeList = append(routeList, staticRoute{uint8(prefixSize), destinationAddress, routerAddress})
		offset = entryEnd
	}
	return routeList, nil
}

type domainListOption struct {
	option
}

const pointerPrefix = '\xC0'

func (o domainListOption) pack(value interface{}) ([]byte, error) {
	domainList, ok := value.([]string)
	if !ok {
		return nil, errors.New("expected string slice")
	}
	byteStr := ""
	for _, domain := range domainList {
		for _, part := range strings.Split(domain, ".") {
			byteStr += string([]byte{uint8(len(part))})
			byteStr += part
		}
		byteStr += "\x00"
	}
	return []byte(byteStr), nil
}

func (o domainListOption) unpack(bytes []byte) (interface{}, error) {
	var domainList []string
	offset := 0
	for offset < len(bytes) {
		newOffset, domainParts, err := o.readDomainName(bytes, offset)
		if err != nil {
			return nil, err
		}
		domainName := strings.Join(domainParts, ".")
		domainList = append(domainList, domainName)
		if newOffset <= offset {
			return nil, errors.New("parsing logic error is letting domain list parsing go on forever")
		}
		offset = newOffset
	}
	return domainList, nil
}

func (o domainListOption) readDomainName(bytes []byte, offset int) (int, []string, error) {
	var parts []string
	for {
		if offset >= len(bytes) {
			return 0, nil, errors.New("domain list ended without a NULL byte")
		}
		maybePartLen := int(bytes[offset])
		offset++
		if maybePartLen == 0 {
			return offset, parts, nil
		} else if (maybePartLen & pointerPrefix) == pointerPrefix {
			if offset >= len(bytes) {
				return 0, nil, errors.New("missing second byte of domain suffix pointer")
			}
			maybePartLen &= ^pointerPrefix
			pointerOffset := ((maybePartLen << 8) + int(bytes[offset]))
			offset++
			_, moreParts, err := o.readDomainName(bytes, pointerOffset)
			if err != nil {
				return 0, nil, err
			}
			parts = append(parts, moreParts...)
			return offset, parts, nil
		} else {
			partLen := maybePartLen
			if offset+partLen >= len(bytes) {
				return 0, nil, errors.New("part of a domain goes beyond data length")
			}
			parts = append(parts, string(bytes[offset:offset+partLen]))
			offset += partLen
		}
	}
}

type fieldInterface interface {
	pack(interface{}) ([]byte, error)
	unpack([]byte) (interface{}, error)
	name() string
	offset() int
	size() int
}

type field struct {
	nameField   string
	offsetField int
	sizeField   int
}

func (f field) name() string {
	return f.nameField
}

func (f field) offset() int {
	return f.offsetField
}

func (f field) size() int {
	return f.sizeField
}

type byteField struct {
	field
}

func (f byteField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return nil, errors.New("expected uint8")
	}
	return []byte{valInt}, nil
}

func (f byteField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(bytes[0]), nil
}

type shortField struct {
	field
}

func (f shortField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint16)
	if !ok {
		return nil, errors.New("expected uint16")
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, valInt)
	return buf, nil
}

func (f shortField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16(bytes), nil
}

type intField struct {
	field
}

func (f intField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint32)
	if !ok {
		return nil, errors.New("expected uint32")
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, valInt)
	return buf, nil
}

func (f intField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32(bytes), nil
}

type HWAddrField struct {
	field
}

func (f HWAddrField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 16 {
		return nil, errors.New("expected string of length no more than 16")
	}
	valStr += strings.Repeat("\x00", 16-len(valStr))
	return []byte(valStr), nil
}

func (f HWAddrField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 16 {
		return nil, errors.New("expected byte slice of length 16")
	}
	return string(bytes), nil
}

type serverNameField struct {
	field
}

func (f serverNameField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 64 {
		return nil, errors.New("expected string of length no more than 64")
	}
	valStr += strings.Repeat("\x00", 64-len(valStr))
	return []byte(valStr), nil
}

func (f serverNameField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 64 {
		return nil, errors.New("expected byte slice of length 64")
	}
	return string(bytes), nil
}

type bootFileField struct {
	field
}

func (f bootFileField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 128 {
		return nil, errors.New("expected string of length no more than 128")
	}
	valStr += strings.Repeat("\x00", 128-len(valStr))
	return []byte(valStr), nil
}

func (f bootFileField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 128 {
		return nil, errors.New("expected byte slice of length 128")
	}
	return string(bytes), nil
}

type IPAddressField struct {
	field
}

func (f IPAddressField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return IPToBytes(valStr)
}

func (f IPAddressField) unpack(bytes []byte) (interface{}, error) {
	return bytesToIP(bytes)
}

type messageType struct {
	name        string
	optionValue uint8
}

var (
	fieldOp             = byteField{field{"op", 0, 1}}
	fieldHWType         = byteField{field{"htype", 1, 1}}
	fieldHWAddrLen      = byteField{field{"hlen", 2, 1}}
	fieldRelayHops      = byteField{field{"hops", 3, 1}}
	fieldTransactionID  = intField{field{"xid", 4, 4}}
	fieldTimeSinceStart = shortField{field{"secs", 8, 2}}
	fieldFlags          = shortField{field{"flags", 10, 2}}
	fieldClientIP       = IPAddressField{field{"ciaddr", 12, 4}}
	fieldYourIP         = IPAddressField{field{"yiaddr", 16, 4}}
	fieldServerIP       = IPAddressField{field{"siaddr", 20, 4}}
	fieldGatewayIP      = IPAddressField{field{"giaddr", 24, 4}}
	fieldClientHWAddr   = HWAddrField{field{"chaddr", 28, 16}}

	fieldLegacyServerName = serverNameField{field{"servername", 44, 64}}
	fieldLegacyBootFile   = bootFileField{field{"bootfile", 108, 128}}
	fieldMagicCookie      = intField{field{"magic_cookie", 236, 4}}
)

var (
	optionTimeOffset                = intOption{option{"time_offset", 2}}
	optionRouters                   = IPListOption{option{"routers", 3}}
	optionSubnetMask                = IPAddressOption{option{"subnet_mask", 1}}
	optionTimeServers               = IPListOption{option{"time_servers", 4}}
	optionNameServers               = IPListOption{option{"name_servers", 5}}
	optionDNSServers                = IPListOption{option{"dns_servers", 6}}
	optionLogServers                = IPListOption{option{"log_servers", 7}}
	optionCookieServers             = IPListOption{option{"cookie_servers", 8}}
	optionLPRServers                = IPListOption{option{"lpr_servers", 9}}
	optionImpressServers            = IPListOption{option{"impress_servers", 10}}
	optionResourceLOCServers        = IPListOption{option{"resource_loc_servers", 11}}
	optionHostName                  = rawOption{option{"host_name", 12}}
	optionBootFileSize              = shortOption{option{"boot_file_size", 13}}
	optionMeritDumpFile             = rawOption{option{"merit_dump_file", 14}}
	optionDomainName                = rawOption{option{"domain_name", 15}}
	optionSwapServer                = IPAddressOption{option{"swap_server", 16}}
	optionRootPath                  = rawOption{option{"root_path", 17}}
	optionExtensions                = rawOption{option{"extensions", 18}}
	optionInterfaceMTU              = shortOption{option{"interface_mtu", 26}}
	optionVendorEncapsulatedOptions = rawOption{option{"vendor_encapsulated_options", 43}}
	optionRequestedIP               = IPAddressOption{option{"requested_ip", 50}}
	optionIPLeaseTime               = intOption{option{"ip_lease_time", 51}}
	optionOptionOverload            = byteOption{option{"option_overload", 52}}
	optionDHCPMessageType           = byteOption{option{"dhcp_message_type", 53}}
	optionServerID                  = IPAddressOption{option{"server_id", 54}}
	optionParameterRequestList      = byteListOption{option{"parameter_request_list", 55}}
	optionMessage                   = rawOption{option{"message", 56}}
	optionMaxDHCPMessageSize        = shortOption{option{"max_dhcp_message_size", 57}}
	optionRenewalT1TimeValue        = intOption{option{"renewal_t1_time_value", 58}}
	optionRebindingT2TimeValue      = intOption{option{"rebinding_t2_time_value", 59}}
	optionVendorID                  = rawOption{option{"vendor_id", 60}}
	optionClientID                  = rawOption{option{"client_id", 61}}
	optionTFTPServerName            = rawOption{option{"tftp_server_name", 66}}
	optionBootfileName              = rawOption{option{"bootfile_name", 67}}
	optionFullyQualifiedDomainName  = rawOption{option{"fqdn", 81}}
	optionDNSDomainSearchList       = domainListOption{option{"domain_search_list", 119}}
	optionClasslessStaticRoutes     = classlessStaticRoutesOption{option{"classless_static_routes", 121}}
	optionWebProxyAutoDiscovery     = rawOption{option{"wpad", 252}}
)

var (
	messageTypeUnknown   = messageType{"UNKNOWN", 0}
	messageTypeDiscovery = messageType{"DISCOVERY", 1}
	messageTypeOffer     = messageType{"OFFER", 2}
	messageTypeRequest   = messageType{"REQUEST", 3}
	messageTypeDecline   = messageType{"DECLINE", 4}
	messageTypeAck       = messageType{"ACK", 5}
	messageTypeNAK       = messageType{"NAK", 6}
	messageTypeRelease   = messageType{"RELEASE", 7}
	messageTypeInform    = messageType{"INFORM", 8}
)

const (
	DHCPMinPacketSize = 300
	IPv4NullAddress   = "0.0.0.0"

	optionPad          = 0
	optionEnd          = uint8(255)
	optionsStartOffset = 240

	fieldValueOpClientRequest  = uint8(1)
	fieldValueOpServerResponse = uint8(2)

	fieldValueHWType10MBEth = uint8(1)

	fieldValueHWAddrLen10MBEth = uint8(6)
	fieldValueMagicCookie      = uint32(0x63825363)
)

var (
	DHCPCommonFields = []fieldInterface{
		fieldOp,
		fieldHWType,
		fieldHWAddrLen,
		fieldRelayHops,
		fieldTransactionID,
		fieldTimeSinceStart,
		fieldFlags,
		fieldClientIP,
		fieldYourIP,
		fieldServerIP,
		fieldGatewayIP,
		fieldClientHWAddr,
	}

	DHCPRequiredFields = append(DHCPCommonFields, fieldMagicCookie)

	DHCPAllFields = append(DHCPCommonFields, []fieldInterface{fieldLegacyServerName, fieldLegacyBootFile, fieldMagicCookie}...)

	DHCPPacketOptions = []optionInterface{
		optionTimeOffset,
		optionRouters,
		optionSubnetMask,
		optionTimeServers,
		optionNameServers,
		optionDNSServers,
		optionLogServers,
		optionCookieServers,
		optionLPRServers,
		optionImpressServers,
		optionResourceLOCServers,
		optionHostName,
		optionBootFileSize,
		optionMeritDumpFile,
		optionSwapServer,
		optionDomainName,
		optionRootPath,
		optionExtensions,
		optionInterfaceMTU,
		optionVendorEncapsulatedOptions,
		optionRequestedIP,
		optionIPLeaseTime,
		optionOptionOverload,
		optionDHCPMessageType,
		optionServerID,
		optionParameterRequestList,
		optionMessage,
		optionMaxDHCPMessageSize,
		optionRenewalT1TimeValue,
		optionRebindingT2TimeValue,
		optionVendorID,
		optionClientID,
		optionTFTPServerName,
		optionBootfileName,
		optionFullyQualifiedDomainName,
		optionDNSDomainSearchList,
		optionClasslessStaticRoutes,
		optionWebProxyAutoDiscovery,
	}

	messageTypeByNum = []messageType{
		messageTypeUnknown,
		messageTypeDiscovery,
		messageTypeOffer,
		messageTypeRequest,
		messageTypeDecline,
		messageTypeAck,
		messageTypeNAK,
		messageTypeRelease,
		messageTypeInform,
	}

	optionValueParameterRequestListDefault = []uint8{
		optionRequestedIP.number(),
		optionIPLeaseTime.number(),
		optionServerID.number(),
		optionSubnetMask.number(),
		optionRouters.number(),
		optionDNSServers.number(),
		optionHostName.number(),
	}
)

func getDHCPOptionByNumber(number uint8) *optionInterface {
	for _, option := range DHCPPacketOptions {
		if option.number() == number {
			return &option
		}
	}
	return nil
}

type DHCPPacket struct {
	options map[optionInterface]interface{}
	fields  map[fieldInterface]interface{}
}

func createDiscoveryPacket(macAddr string) (*DHCPPacket, error) {
	macAddr += strings.Repeat(string([]byte{optionPad}), 12-len(macAddr))
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpClientRequest)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, rand.Uint32())
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, IPv4NullAddress)
	packet.setField(fieldYourIP, IPv4NullAddress)
	packet.setField(fieldServerIP, IPv4NullAddress)
	packet.setField(fieldGatewayIP, IPv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, messageTypeDiscovery.optionValue)
	return packet, nil
}

func createOfferPacket(transactionID uint32, macAddr string, offerIP string, serverIP string) (*DHCPPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpServerResponse)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, IPv4NullAddress)
	packet.setField(fieldYourIP, offerIP)
	packet.setField(fieldServerIP, serverIP)
	packet.setField(fieldGatewayIP, IPv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, messageTypeOffer.optionValue)
	return packet, nil
}

func createRequestPacket(transactionID uint32, macAddr string) (*DHCPPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpClientRequest)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, IPv4NullAddress)
	packet.setField(fieldYourIP, IPv4NullAddress)
	packet.setField(fieldServerIP, IPv4NullAddress)
	packet.setField(fieldGatewayIP, IPv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, messageTypeRequest.optionValue)
	return packet, nil
}

func createAcknowledgementPacket(transactionID uint32, macAddr string, grantedIP string, serverIP string) (*DHCPPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpServerResponse)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, IPv4NullAddress)
	packet.setField(fieldYourIP, grantedIP)
	packet.setField(fieldServerIP, serverIP)
	packet.setField(fieldGatewayIP, IPv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, messageTypeAck.optionValue)
	return packet, nil
}

func createNAKPacket(transactionID uint32, macAddr string) (*DHCPPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpServerResponse)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, IPv4NullAddress)
	packet.setField(fieldYourIP, IPv4NullAddress)
	packet.setField(fieldServerIP, IPv4NullAddress)
	packet.setField(fieldGatewayIP, IPv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, messageTypeNAK.optionValue)
	return packet, nil
}

func newDHCPPacket(bytes []byte) (*DHCPPacket, error) {
	var packet DHCPPacket
	packet.options = make(map[optionInterface]interface{})
	packet.fields = make(map[fieldInterface]interface{})
	if len(bytes) == 0 {
		return &packet, nil
	}
	if len(bytes) < optionsStartOffset+1 {
		return nil, errors.New("invalid byte string for packet")
	}
	for _, field := range DHCPAllFields {
		fieldVal, err := field.unpack(bytes[field.offset() : field.offset()+field.size()])
		if err != nil {
			return nil, err
		}
		packet.fields[field] = fieldVal
	}
	offset := optionsStartOffset
	var domainSearchListByteString []byte
	for offset < len(bytes) && bytes[offset] != optionEnd {
		dataType := bytes[offset]
		offset++
		if dataType == optionEnd {
			continue
		}
		dataLength := int(bytes[offset])
		offset++
		data := bytes[offset : offset+dataLength]
		offset += dataLength
		option := getDHCPOptionByNumber(dataType)
		if option == nil {
			continue
		}
		if *option == optionDNSDomainSearchList {
			domainSearchListByteString = append(domainSearchListByteString, data...)
			continue
		}
		optionVal, err := (*option).unpack(data)
		if err != nil {
			return nil, err
		}
		optionValue := optionVal
		packet.options[*option] = optionValue
	}
	if len(domainSearchListByteString) > 0 {
		domainSearchListVal, err := optionDNSDomainSearchList.unpack(domainSearchListByteString)
		if err != nil {
			return nil, err
		}
		packet.options[optionDNSDomainSearchList] = domainSearchListVal
	}
	return &packet, nil
}

func (d *DHCPPacket) clientHWAddress() (string, error) {
	addr, ok := d.fields[fieldClientHWAddr]
	if !ok {
		return "", errors.New("client addr field not found")
	}
	addrStr, ok := addr.(string)
	if !ok {
		return "", errors.New("expected string type")
	}
	return addrStr, nil
}

func (d *DHCPPacket) isValid() bool {
	for _, field := range DHCPRequiredFields {
		if d.fields[field] == nil {
			return false
		}
	}
	if d.fields[fieldMagicCookie] != fieldValueMagicCookie {
		return false
	}
	return true
}

func (d *DHCPPacket) messageType() (messageType, error) {
	typeNum, ok := d.options[optionDHCPMessageType]
	if !ok {
		return messageTypeUnknown, errors.New("message type option not found")
	}
	typeNumInt, ok := typeNum.(uint8)
	if !ok {
		return messageTypeUnknown, errors.New("expected uint8 type")
	}
	if typeNumInt > 0 && int(typeNumInt) < len(messageTypeByNum) {
		return messageTypeByNum[typeNumInt], nil
	}
	return messageTypeUnknown, errors.New("invalid message type")
}

func (d *DHCPPacket) transactionID() (uint32, error) {
	ID, ok := d.fields[fieldTransactionID]
	if !ok {
		return 0, errors.New("transaction ID field not found")
	}
	IDInt, ok := ID.(uint32)
	if !ok {
		return 0, errors.New("expected uint32 type")
	}
	return IDInt, nil
}

func (d *DHCPPacket) getField(field fieldInterface) interface{} {
	return d.fields[field]
}

func (d *DHCPPacket) getOption(option optionInterface) interface{} {
	return d.options[option]
}

func (d *DHCPPacket) setField(field fieldInterface, fieldValue interface{}) {
	d.fields[field] = fieldValue
}

func (d *DHCPPacket) setOption(option optionInterface, optionValue interface{}) {
	d.options[option] = optionValue
}

func (d *DHCPPacket) toBinaryString() ([]byte, error) {
	if !d.isValid() {
		return nil, errors.New("invalid packet")
	}
	var data []byte
	offset := 0
	for _, field := range DHCPAllFields {
		fieldValue, ok := d.fields[field]
		if !ok {
			continue
		}
		fieldData, err := field.pack(fieldValue)
		if err != nil {
			return nil, err
		}
		for offset < field.offset() {
			data = append(data, '\x00')
			offset++
		}
		data = append(data, fieldData...)
		offset += field.size()
	}
	for _, option := range DHCPPacketOptions {
		optionValue, ok := d.options[option]
		if !ok {
			continue
		}
		serializedValue, err := option.pack(optionValue)
		if err != nil {
			return nil, err
		}
		data = append(data, option.number(), uint8(len(serializedValue)))
		offset += 2
		data = append(data, serializedValue...)
		offset += len(serializedValue)
	}
	data = append(data, optionEnd)
	offset++
	for offset < DHCPMinPacketSize {
		data = append(data, optionPad)
		offset++
	}
	return data, nil
}

func (d *DHCPPacket) String() string {
	var options, fields []string
	for field, fieldVal := range d.fields {
		fieldStr := fmt.Sprintf("%v=%v", field.name(), fieldVal)
		fields = append(fields, fieldStr)
	}
	for option, optionVal := range d.options {
		optionStr := fmt.Sprintf("%v=%v", option.name(), optionVal)
		options = append(options, optionStr)
	}
	return fmt.Sprintf("<DHCPPacket fields=[%s], options=[%s]>", strings.Join(fields, ","), strings.Join(options, ","))
}
