// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <stdio.h>

#include <memory>
#include <string>

#include <base/callback.h>
#include <base/check_op.h>
#include <base/files/file_descriptor_watcher_posix.h>
#include <base/json/json_writer.h>
#include <base/logging.h>
#include <base/run_loop.h>
#include <base/task/single_thread_task_executor.h>
#include <base/test/bind.h>
#include <base/values.h>
#include <dbus/bus.h>

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

  root_dict.Set("FeatureName", feature_state.feature_name);
  root_dict.Set("EnabledCallbackEnabledResult",
                feature_state.enabled_callback_enabled_result);

  auto result_entry =
      feature_state.params_callback_result.find(feature_state.feature_name);

  CHECK(result_entry != feature_state.params_callback_result.end())
      << "Did not find expected feature";

  std::string name = result_entry->first;
  feature::PlatformFeatures::ParamsResultEntry feature = result_entry->second;
  root_dict.Set("ParamsCallbackFeatureName", std::move(name));
  root_dict.Set("ParamsCallbackEnabledResult", feature.enabled);

  base::Value::Dict params;
  for (const auto& [key, value] : feature.params) {
    params.Set(key, value);
  }
  root_dict.Set("ParamsCallbackParamsResult", base::Value(std::move(params)));

  std::string output_json;
  base::JSONWriter::Write(root_dict, &output_json);
  printf("%s\n", output_json.c_str());
}

bool IsFeatureEnabled(const VariationsFeature& feature_to_check,
                      feature::PlatformFeatures* feature_lib) {
  bool result = false;
  base::RunLoop run_loop;
  feature_lib->IsEnabled(feature_to_check,
                         base::BindLambdaForTesting([&](bool enabled) {
                           result = enabled;
                           run_loop.Quit();
                         }));
  run_loop.Run();
  return result;
}

feature::PlatformFeatures::ParamsResult
GetParamsAndEnabled(const VariationsFeature& feature_to_check,
                    feature::PlatformFeatures* feature_lib) {
  feature::PlatformFeatures::ParamsResult result;
  base::RunLoop run_loop;
  feature_lib->GetParamsAndEnabled(
      {&feature_to_check},
      base::BindLambdaForTesting(
          [&](feature::PlatformFeatures::ParamsResult params) {
            result = params;
            run_loop.Quit();
          }));
  run_loop.Run();
  return result;
}

TestFeatureState
GetTestFeatureStateAndParams(const VariationsFeature& feature_to_check,
                             feature::PlatformFeatures* feature_lib) {
  TestFeatureState feature_state;
  feature_state.feature_name = feature_to_check.name;
  feature_state.enabled_callback_enabled_result =
      IsFeatureEnabled(feature_to_check, feature_lib);
  feature_state.params_callback_result =
      GetParamsAndEnabled(feature_to_check, feature_lib);
  return feature_state;
}

int main(int argc, char* argv[]) {
  base::SingleThreadTaskExecutor task_executor(base::MessagePumpType::IO);
  base::FileDescriptorWatcher watcher(task_executor.task_runner());

  dbus::Bus::Options options;
  options.bus_type = dbus::Bus::SYSTEM;
  scoped_refptr<dbus::Bus> bus(new dbus::Bus(options));

  std::unique_ptr<feature::PlatformFeatures> feature_lib =
      feature::PlatformFeatures::New(bus);

  TestFeatureState enabled_feature = GetTestFeatureStateAndParams(
      kCrOSLateBootDefaultEnabled, feature_lib.get());
  LOG(INFO) << "Finished getting state and params for Default Enabled Feature";

  TestFeatureState disabled_feature = GetTestFeatureStateAndParams(
      kCrOSLateBootDefaultDisabled, feature_lib.get());
  LOG(INFO) << "Finished getting state and params for Default Disabled Feature";

  LogTestFeatureState(enabled_feature);
  LOG(INFO) << "Finished logging Default Enabled Feature";
  LogTestFeatureState(disabled_feature);
  LOG(INFO) << "Finished logging Default Disabled Feature";
}
