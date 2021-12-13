// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <stdio.h>
#include <string.h>

#include <iostream>
#include <queue>
#include <sstream>

#include "ppapi/c/pp_errors.h"
#include "ppapi/c/ppb_console.h"
#include "ppapi/cpp/instance.h"
#include "ppapi/cpp/module.h"
#include "ppapi/cpp/var.h"
#include "ppapi/utility/completion_callback_factory.h"
#include "testdata.h"

// Use assert as a makeshift CHECK, even in non-debug mode.
// Since <assert.h> redefines assert on every inclusion (it doesn't use
// include-guards), make sure this is the last file #include'd in this file.
#undef NDEBUG
#include <assert.h>

namespace {

const int kValidMessage = 1;
const int kValidReply = 2;

class MyInstance;

class MyInstance : public pp::Instance {
 public:
  MyInstance(PP_Instance instance);
  virtual ~MyInstance();

  // pp::Instance implementation.
  virtual void HandleMessage(const pp::Var &message);
};

MyInstance::MyInstance(PP_Instance instance)
    : pp::Instance(instance) {
    }

MyInstance::~MyInstance() {}

void MyInstance::HandleMessage(const pp::Var &message) {
  if (!message.is_int()) {
    return; // TODO future: actually log something or some such
  }
  int value = message.AsInt();
  
  if(value == kValidMessage) {
    pp::Var reply = pp::Var(kValidReply);
    PostMessage(reply);
  }
}

// This object is the global object representing this plugin library as long
// as it is loaded.
class MyModule : public pp::Module {
 public:
  MyModule() : pp::Module() {}
  virtual ~MyModule() {}

  virtual pp::Instance* CreateInstance(PP_Instance instance) {
    return new MyInstance(instance);
  }
};

}  // anonymous namespace

namespace pp {
// Factory function for your specialization of the Module object.
Module* CreateModule() {
  return new MyModule();
}
}  // namespace pp
