// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	empb "chromiumos/policy/chromium/policy/enterprise_management_proto"
	"chromiumos/tast/errors"
)

// getStringVal marshals the struct into string value.
func getStringVal(val interface{}) string {
	var tmp string
	switch v := val.(type) {
	case string:
		tmp = v
	default:
		out, err := json.Marshal(v)
		if err != nil {
			panic(errors.Wrap(err, "couldn't marshal the policy value to json"))
		}
		tmp = string(out)
	}
	return tmp
}

// setProtobufMessageField sets a field of simple type such as StringPolicyProto, StringListPolicyProto, BooleanPolicyProto or IntegerPolicyProto to the policy name with the given value.
func setProtobufMessageField(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
	if policyName == "device_local_accounts" && fieldName == "account" {
		setDeviceLocalAccountsProto(m, policyName, fieldName, val)
		return
	}
	protoMessageDesc := (*m).Descriptor().Fields().ByName(protoreflect.Name(policyName))
	fieldMessage := (*m).Get(protoMessageDesc).Message()
	if !fieldMessage.IsValid() {
		fieldMessage = fieldMessage.New()
	}
	valueDesc := fieldMessage.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	switch fieldMessage.Type().Descriptor().Name() {
	case "StringPolicyProto":
		fieldMessage.Set(valueDesc, protoreflect.ValueOf(getStringVal(val)))
	case "StringListPolicyProto":
		valueMessage := (fieldMessage).Get(valueDesc).Message()
		if !valueMessage.IsValid() {
			valueMessage = valueMessage.New()
		}
		entriesDesc := valueMessage.Descriptor().Fields().ByName("entries")
		if !entriesDesc.IsList() {
			panic("the entries are not list")
		}
		components := valueMessage.NewField(entriesDesc).List()
		if !components.IsValid() {
			panic(fmt.Sprintf("entries message isn't valid: %v", components))
		}
		for _, vv := range (val).([]string) {
			components.Append(protoreflect.ValueOf(vv))
		}
		valueMessage.Set(entriesDesc, protoreflect.ValueOf(components))

		fieldMessage.Set(valueDesc, protoreflect.ValueOf(valueMessage))
	default:
		switch val.(type) {
		case int:
			fieldMessage.Set(valueDesc, protoreflect.ValueOf(int32(val.(int))))
		default:
			fieldMessage.Set(valueDesc, protoreflect.ValueOf(val))
		}
	}
	(*m).Set(protoMessageDesc, protoreflect.ValueOfMessage(fieldMessage))
}

// setDeviceLocalAccountsProto sets all the required fields of DeviceLocalAccountsProto.
func setDeviceLocalAccountsProto(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
	deviceLocalAccountsInfoValue := val.([]DeviceLocalAccountInfo)
	protoMessageDesc := (*m).Descriptor().Fields().ByName(protoreflect.Name(policyName))
	fieldMessage := (*m).Get(protoMessageDesc).Message()
	if !fieldMessage.IsValid() {
		fieldMessage = fieldMessage.New()
	}
	accountDesc := fieldMessage.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if !accountDesc.IsList() {
		panic("the accounts are not list")
	}
	accounts := fieldMessage.NewField(accountDesc).List()
	if !accounts.IsValid() {
		panic(fmt.Sprintf("account message isn't valid: %v", accounts))
	}

	for _, v := range deviceLocalAccountsInfoValue {
		deviceLocalAccountProto := empb.DeviceLocalAccountInfoProto{}
		if v.AccountID != nil {
			deviceLocalAccountProto.AccountId = v.AccountID
		}
		if v.AccountType != nil {
			deviceLocalAccountProto.Type = &[]empb.DeviceLocalAccountInfoProto_AccountType{empb.DeviceLocalAccountInfoProto_AccountType(*v.AccountType)}[0]
		}

		if v.KioskAppInfo != nil {
			kioskAppProto := empb.KioskAppInfoProto{}
			kioskAppProto.AppId = v.KioskAppInfo.AppId
			kioskAppProto.UpdateUrl = v.KioskAppInfo.UpdateUrl
			deviceLocalAccountProto.KioskApp = &kioskAppProto
		}

		if v.AndroidKioskAppInfo != nil {
			androidKioskProto := empb.AndroidKioskAppInfoProto{}
			androidKioskProto.PackageName = v.AndroidKioskAppInfo.PackageName
			androidKioskProto.ClassName = v.AndroidKioskAppInfo.ClassName
			androidKioskProto.DisplayName = v.AndroidKioskAppInfo.DisplayName
			androidKioskProto.Action = v.AndroidKioskAppInfo.Action
			deviceLocalAccountProto.AndroidKioskApp = &androidKioskProto
		}

		if v.WebKioskAppInfo != nil {
			webKioskAppProto := empb.WebKioskAppInfoProto{}
			webKioskAppProto.Url = v.WebKioskAppInfo.Url
			webKioskAppProto.Title = v.WebKioskAppInfo.Title
			webKioskAppProto.IconUrl = v.WebKioskAppInfo.IconUrl
			deviceLocalAccountProto.WebKioskApp = &webKioskAppProto
		}
		accounts.Append(protoreflect.ValueOfMessage(deviceLocalAccountProto.ProtoReflect()))
	}

	fieldMessage.Set(accountDesc, protoreflect.ValueOf(accounts))
	(*m).Set(protoMessageDesc, protoreflect.ValueOfMessage(fieldMessage))
}

// SetUserProto sets the specified field of the user policy proto.
func SetUserProto(m *protoreflect.Message, policyName string, val interface{}) {
	setProtobufMessageField(m, policyName, "value", val)
}

// SetDeviceProto sets the specified field of the device policy proto.
func SetDeviceProto(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
	setProtobufMessageField(m, policyName, fieldName, val)
}
