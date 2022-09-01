// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
#include <base/callback.h>
#include <base/files/file_descriptor_watcher_posix.h>
#include <base/json/json_writer.h>
#include <base/logging.h>
#include <base/run_loop.h>
#include <base/task/single_thread_task_executor.h>
#include <base/values.h>
#include <dbus/bus.h>

#include <string>

#include "featured/feature_library.h"

const struct VariationsFeature kCrOSLateBootDefaultEnabled = {
    .name = "CrOSLateBootTestDefaultEnabled",
    .default_state = FEATURE_ENABLED_BY_DEFAULT,
};

const struct VariationsFeature kCrOSLateBootDefaultDisabled = {
    .name = "CrOSLateBootTestDefaultDisabled",
    .default_state = FEATURE_DISABLED_BY_DEFAULT,
};

struct TestFeatureState {
  std::string FeatureName;
  bool EnabledCallbackEnabledResult;
  std::string ParamsCallbackFeatureName;
  bool ParamsCallbackEnabledResult;
  feature::PlatformFeatures::ParamsResult ParamsCallbackParamsResult;
};

void LogTestFeatureState(TestFeatureState feature) {
  std::string output_js;
  base::Value::Dict root_dict;
  base::Value::List params;

  root_dict.Set("FeatureName:", feature.FeatureName);
  root_dict.Set("EnabledCallbackEnabledResult",
                feature.EnabledCallbackEnabledResult);
  root_dict.Set("ParamsCallbackFeatureName",
                feature.ParamsCallbackFeatureName);
  root_dict.Set("ParamsCallbackEnabledResult",
                feature.ParamsCallbackEnabledResult);

  for (const auto& [name, entry]: feature.ParamsCallbackParamsResult) {
    for (const auto& [key, value] : entry.params) {
      base::Value::Dict param;
      param.Set(key, value);
      params.Append(std::move(param));
    }
  }
  root_dict.Set("ParamsCallbackParamsResult", base::Value(std::move(params)));

  base::JSONWriter::Write(root_dict, &output_js);
  printf("%s\n", output_js.c_str());
}

void EnabledCallback(base::RepeatingClosure quit_closure,
                      TestFeatureState* feature,
                      bool enabled) {
  feature->EnabledCallbackEnabledResult = enabled;
  quit_closure.Run();
}

void GetParamsCallback(base::RepeatingClosure quit_closure,
                        TestFeatureState* feature,
                        feature::PlatformFeatures::ParamsResult result) {
  for (const auto& [name, entry] : result) {
    feature->ParamsCallbackFeatureName = name;
    feature->ParamsCallbackEnabledResult = entry.enabled;
    feature->ParamsCallbackParamsResult = result;
  }
  quit_closure.Run();
}

void IsFeatureEnabled(const VariationsFeature& kCrOSLateBootFeature,
                      feature::PlatformFeatures* feature_lib,
                      TestFeatureState* feature) {
  base::RunLoop run_loop;
  auto quit_closure = run_loop.QuitClosure();

  feature_lib->IsEnabled(kCrOSLateBootFeature,
                          base::BindOnce(&EnabledCallback,
                                          quit_closure,
                                          feature));
  run_loop.Run();
}

void GetParamsAndEnabled(const VariationsFeature& kCrOSLateBootFeature,
                          feature::PlatformFeatures* feature_lib,
                          TestFeatureState* feature) {
  base::RunLoop run_loop;
  auto quit_closure = run_loop.QuitClosure();
  feature_lib->GetParamsAndEnabled(
      {&kCrOSLateBootFeature},
      base::BindOnce(&GetParamsCallback, quit_closure, feature));
  run_loop.Run();
}

void GetTestFeatureStateAndParams(const VariationsFeature& kCrOSLateBootFeature,
                                  feature::PlatformFeatures* feature_lib,
                                  TestFeatureState* feature) {
  feature->FeatureName = kCrOSLateBootFeature.name;
  IsFeatureEnabled(kCrOSLateBootFeature, feature_lib, feature);
  GetParamsAndEnabled(kCrOSLateBootFeature, feature_lib, feature);
}

int main(int argc, char* argv[]) {
  base::SingleThreadTaskExecutor task_executor(base::MessagePumpType::IO);
  base::FileDescriptorWatcher watcher(task_executor.task_runner());

  dbus::Bus::Options options;
  options.bus_type = dbus::Bus::SYSTEM;
  scoped_refptr<dbus::Bus> bus(new dbus::Bus(options));

  std::unique_ptr<feature::PlatformFeatures> feature_lib =
      feature::PlatformFeatures::New(bus);

  TestFeatureState EnabledFeature;
  TestFeatureState DisabledFeature;

  GetTestFeatureStateAndParams(kCrOSLateBootDefaultEnabled,
                                feature_lib.get(),
                                &EnabledFeature);
  GetTestFeatureStateAndParams(kCrOSLateBootDefaultDisabled,
                                feature_lib.get(),
                                &DisabledFeature);

  LogTestFeatureState(EnabledFeature);
  LogTestFeatureState(DisabledFeature);
}