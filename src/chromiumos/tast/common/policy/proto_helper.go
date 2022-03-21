// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

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

// SetProtoField sets the specified field of the proto.
func SetProtoField(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
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
			panic(fmt.Sprintf("components is invalid: %v", components))
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

// SetUserProto sets the specified field of the user policy proto.
func SetUserProto(m *protoreflect.Message, policyName string, val interface{}) {
	SetProtoField(m, policyName, "value", val)
}

// SetDeviceProto sets the specified field of the device policy proto.
func SetDeviceProto(m *protoreflect.Message, policyName, fieldName string, val interface{}) {
	SetProtoField(m, policyName, fieldName, val)
}
