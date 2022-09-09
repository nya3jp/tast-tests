/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.accessibilitytest;

import android.app.Activity;
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
                    public void onInitializeAccessibilityNodeInfo(
                            View host, AccessibilityNodeInfo info) {
                        super.onInitializeAccessibilityNodeInfo(host, info);
                        info.addAction(
                                new AccessibilityAction(
                                        AccessibilityAction.ACTION_CLICK.getId(), "perform click"));
                        info.addAction(
                                new AccessibilityAction(
                                        AccessibilityAction.ACTION_LONG_CLICK.getId(),
                                        "perform long click"));
                    }
                });

        Button customActionButton = findViewById(R.id.customActionButton);
        customActionButton.setAccessibilityDelegate(
                new AccessibilityDelegate() {
                    @Override
                    public void onInitializeAccessibilityNodeInfo(
                            View host, AccessibilityNodeInfo info) {
                        super.onInitializeAccessibilityNodeInfo(host, info);
                        info.addAction(
                                new AccessibilityNodeInfo.AccessibilityAction(
                                        R.id.custom_action, "perform custom action"));
                    }

                    @Override
                    public boolean performAccessibilityAction(View host, int action, Bundle args) {
                        switch (action) {
                            case R.id.custom_action:
                                host.announceForAccessibility("custom action performed");
                                return true;
                            default:
                                return super.performAccessibilityAction(host, action, args);
                        }
                    }
                });
    }
}
