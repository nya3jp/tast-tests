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
  std::string feature_name;
  bool enabled_callback_enabled_result;
  feature::PlatformFeatures::ParamsResult params_callback_result;
};

void LogTestFeatureState(const TestFeatureState& feature_state) {
  base::Value::Dict root_dict;

  root_dict.Set("FeatureName:", feature_state.feature_name);
  root_dict.Set("EnabledCallbackEnabledResult",
                feature_state.enabled_callback_enabled_result);



  base::Value::Dict params;

  auto result_entry = feature_state.params_callback_result
                      .find(feature_state.feature_name);

  if (result_entry == feature_state.params_callback_result.end()) {
    exit(1);
  }

  std::string name = result_entry->first;
  feature::PlatformFeatures::ParamsResultEntry feature = result_entry->second;
  root_dict.Set("ParamsCallbackFeatureName", name);
  root_dict.Set("ParamsCallbackEnabledResult", feature.enabled);
  for (const auto& [key, value] : feature.params) {
    params.Set(key, value);
  }
  root_dict.Set("ParamsCallbackParamsResult", base::Value(std::move(params)));

  std::string output_json;
  base::JSONWriter::Write(root_dict, &output_json);
  printf("%s\n", output_json.c_str());
}

void EnabledCallback(base::RepeatingClosure quit_closure,
                      TestFeatureState* feature_state,
                      bool enabled) {
  feature_state->enabled_callback_enabled_result = enabled;
  quit_closure.Run();
}

void GetParamsCallback(base::RepeatingClosure quit_closure,
                        TestFeatureState* feature_state,
                        feature::PlatformFeatures::ParamsResult result) {
  feature_state->params_callback_result = result;
  quit_closure.Run();
}

void IsFeatureEnabled(const VariationsFeature& feature_to_check,
                      feature::PlatformFeatures* feature_lib,
                      TestFeatureState* feature_state) {
  base::RunLoop run_loop;
  auto quit_closure = run_loop.QuitClosure();

  feature_lib->IsEnabled(feature_to_check,
                          base::BindOnce(&EnabledCallback,
                                          quit_closure,
                                          feature_state));
  run_loop.Run();
}

void GetParamsAndEnabled(const VariationsFeature& feature_to_check,
                          feature::PlatformFeatures* feature_lib,
                          TestFeatureState* feature_state) {
  base::RunLoop run_loop;
  auto quit_closure = run_loop.QuitClosure();
  feature_lib->GetParamsAndEnabled(
      {&feature_to_check},
      base::BindOnce(&GetParamsCallback, quit_closure, feature_state));
  run_loop.Run();
}

void GetTestFeatureStateAndParams(const VariationsFeature& feature_to_check,
                                  feature::PlatformFeatures* feature_lib,
                                  TestFeatureState* feature_state) {
  feature_state->feature_name = feature_to_check.name;
  IsFeatureEnabled(feature_to_check, feature_lib, feature_state);
  GetParamsAndEnabled(feature_to_check, feature_lib, feature_state);
}

int main(int argc, char* argv[]) {
  base::SingleThreadTaskExecutor task_executor(base::MessagePumpType::IO);
  base::FileDescriptorWatcher watcher(task_executor.task_runner());

  dbus::Bus::Options options;
  options.bus_type = dbus::Bus::SYSTEM;
  scoped_refptr<dbus::Bus> bus(new dbus::Bus(options));

  std::unique_ptr<feature::PlatformFeatures> feature_lib =
      feature::PlatformFeatures::New(bus);

  TestFeatureState enabled_feature;
  GetTestFeatureStateAndParams(kCrOSLateBootDefaultEnabled,
                                feature_lib.get(),
                                &enabled_feature);

  TestFeatureState disabled_feature;
  GetTestFeatureStateAndParams(kCrOSLateBootDefaultDisabled,
                                feature_lib.get(),
                                &disabled_feature);

  LogTestFeatureState(enabled_feature);
  LogTestFeatureState(disabled_feature);
}
