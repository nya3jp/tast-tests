/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.accessibilitytest;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;
import android.view.View;
import android.view.View.AccessibilityDelegate;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityNodeInfo.AccessibilityAction;
import android.widget.Button;

/** Test Activity containing EditText elements for arc.A11y* tast tests. */
public class ActionActivity extends Activity {
  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);
    setContentView(R.layout.action_activity);

    Button longClickButton = findViewById(R.id.longClickButton);
    AccessibilityNodeInfo longClickInfo = longClickButton.createAccessibilityNodeInfo();
    longClickInfo.removeAction(AccessibilityAction.ACTION_CLICK);
    longClickButton.setOnLongClickListener(
        view -> {
          view.announceForAccessibility("long clicked");
          return true;
        });

    Button labelButton = findViewById(R.id.labelButton);
    labelButton.setAccessibilityDelegate(
        new AccessibilityDelegate() {
          @Override
          public void onInitializeAccessibilityNodeInfo(View host, AccessibilityNodeInfo info) {
            super.onInitializeAccessibilityNodeInfo(host, info);
            info.removeAction(AccessibilityAction.ACTION_CLICK);
            info.addAction(
                new AccessibilityAction(
                    AccessibilityAction.ACTION_CLICK.getId(),
                    "click label"));
            info.removeAction(AccessibilityAction.ACTION_LONG_CLICK);
            info.addAction(
                new AccessibilityAction(
                    AccessibilityAction.ACTION_LONG_CLICK.getId(),
                    "long click label"));
          }
        });
  }
}
