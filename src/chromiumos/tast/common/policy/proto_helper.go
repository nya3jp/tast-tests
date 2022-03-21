// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"chromiumos/policy/chromium/policy/enterprise_management_proto"
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

// SetProtobufMessageField sets the specified field of the proto.
func SetProtobufMessageField(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
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

func setDeviceLocalAccountsProto(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
	v := val.([]DeviceLocalAccountInfo)
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

	for _, vv := range v {
		dlaiProto := enterprise_management_proto.DeviceLocalAccountInfoProto{}
		dlaiProtoMessage := dlaiProto.ProtoReflect().New()

		if vv.AccountID != nil {
			accountIDDesc := dlaiProtoMessage.Descriptor().Fields().ByName("account_id")
			dlaiProtoMessage.Set(accountIDDesc, protoreflect.ValueOf(*vv.AccountID))
		}

		if vv.AccountType != nil {
			accountTypeDesc := dlaiProtoMessage.Descriptor().Fields().ByName("type")
			dlaiProtoMessage.Set(accountTypeDesc, protoreflect.ValueOf(protoreflect.EnumNumber(*vv.AccountType)))
		}

		if vv.KioskAppInfo != nil {
			kioskAppDesc := dlaiProtoMessage.Descriptor().Fields().ByName("kiosk_app")
			kioskAppProto := enterprise_management_proto.KioskAppInfoProto{}
			kioskAppProtoMessage := kioskAppProto.ProtoReflect().New()
			appIDDesc := kioskAppProtoMessage.Descriptor().Fields().ByName("app_id")
			kioskAppProtoMessage.Set(appIDDesc, protoreflect.ValueOf(*vv.KioskAppInfo.AppId))
			updateURLDesc := kioskAppProtoMessage.Descriptor().Fields().ByName("update_url")
			kioskAppProtoMessage.Set(updateURLDesc, protoreflect.ValueOf(*vv.KioskAppInfo.UpdateUrl))
			dlaiProtoMessage.Set(kioskAppDesc, protoreflect.ValueOf(kioskAppProtoMessage))
		}

		if vv.AndroidKioskAppInfo != nil {
			androidKioskAppDesc := dlaiProtoMessage.Descriptor().Fields().ByName("android_kiosk_app")
			androidKioskAppProto := enterprise_management_proto.AndroidKioskAppInfoProto{}
			androidKioskAppProtoMessage := androidKioskAppProto.ProtoReflect().New()
			packageNameDesc := androidKioskAppProtoMessage.Descriptor().Fields().ByName("package_name")
			androidKioskAppProtoMessage.Set(packageNameDesc, protoreflect.ValueOf(*vv.AndroidKioskAppInfo.PackageName))
			classNameDesc := androidKioskAppProtoMessage.Descriptor().Fields().ByName("class_name")
			androidKioskAppProtoMessage.Set(classNameDesc, protoreflect.ValueOf(*vv.AndroidKioskAppInfo.ClassName))
			displayNameDesc := androidKioskAppProtoMessage.Descriptor().Fields().ByName("display_name")
			androidKioskAppProtoMessage.Set(displayNameDesc, protoreflect.ValueOf(*vv.AndroidKioskAppInfo.DisplayName))
			actionDesc := androidKioskAppProtoMessage.Descriptor().Fields().ByName("action")
			androidKioskAppProtoMessage.Set(actionDesc, protoreflect.ValueOf(*vv.AndroidKioskAppInfo.Action))
			dlaiProtoMessage.Set(androidKioskAppDesc, protoreflect.ValueOf(androidKioskAppProtoMessage))
		}

		if vv.WebKioskAppInfo != nil {
			WebKioskAppDesc := dlaiProtoMessage.Descriptor().Fields().ByName("web_kiosk_app")
			webKioskAppProto := enterprise_management_proto.WebKioskAppInfoProto{}
			webKioskAppProtoMessage := webKioskAppProto.ProtoReflect().New()
			urlDesc := webKioskAppProtoMessage.Descriptor().Fields().ByName("url")
			webKioskAppProtoMessage.Set(urlDesc, protoreflect.ValueOf(*vv.WebKioskAppInfo.Url))
			titleDesc := webKioskAppProtoMessage.Descriptor().Fields().ByName("title")
			webKioskAppProtoMessage.Set(titleDesc, protoreflect.ValueOf(*vv.WebKioskAppInfo.Title))
			iconURLDesc := webKioskAppProtoMessage.Descriptor().Fields().ByName("icon_url")
			webKioskAppProtoMessage.Set(iconURLDesc, protoreflect.ValueOf(*vv.WebKioskAppInfo.IconUrl))
			dlaiProtoMessage.Set(WebKioskAppDesc, protoreflect.ValueOf(webKioskAppProtoMessage))
		}

		accounts.Append(protoreflect.ValueOfMessage(dlaiProtoMessage))
	}

	fieldMessage.Set(accountDesc, protoreflect.ValueOf(accounts))
	(*m).Set(protoMessageDesc, protoreflect.ValueOfMessage(fieldMessage))
}

// SetUserProto sets the specified field of the user policy proto.
func SetUserProto(m *protoreflect.Message, policyName string, val interface{}) {
	SetProtobufMessageField(m, policyName, "value", val)
}

// SetDeviceProto sets the specified field of the device policy proto.
func SetDeviceProto(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
	SetProtobufMessageField(m, policyName, fieldName, val)
}
