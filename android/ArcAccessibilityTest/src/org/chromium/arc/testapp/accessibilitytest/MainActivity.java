/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.accessibilitytest;

import android.app.Activity;
import android.content.Context;
import android.os.Build;
import android.os.Bundle;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.CompoundButton;
import android.widget.SeekBar;
import android.widget.Toast;

/** Test Activity for arc.Accessibility* tast tests. */
public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_accessibility);

        Button announceButton = findViewById(R.id.announceButton);
        announceButton.setOnClickListener(
                view -> view.announceForAccessibility("test announcement"));

        Button toastButton = findViewById(R.id.toastButton);
        toastButton.setOnClickListener(
                v -> {
                    Context context = getApplicationContext();
                    CharSequence text = "test toast";
                    int duration = Toast.LENGTH_SHORT;
                    Toast toast = Toast.makeText(context, text, duration);
                    toast.show();
                });

        CheckBox checkBox = findViewById(R.id.checkBoxWithStateDescription);
        if (Build.VERSION.SDK_INT >= 29) {
            checkBox.setStateDescription("state description not checked");
        }
        checkBox.setOnCheckedChangeListener(
                new CompoundButton.OnCheckedChangeListener() {
                    @Override
                    public void onCheckedChanged(CompoundButton buttonView, boolean isChecked) {
                        if (Build.VERSION.SDK_INT < 29) {
                            return;
                        }
                        if (checkBox.isChecked()) {
                            checkBox.setStateDescription("state description checked");
                        } else {
                            checkBox.setStateDescription("state description not checked");
                        }
                    }
                });

        SeekBar seekBar = findViewById(R.id.seekBar);
        if (Build.VERSION.SDK_INT >= 29) {
            seekBar.setStateDescription("state description " + 25);
        }
        seekBar.setOnSeekBarChangeListener(
                new SeekBar.OnSeekBarChangeListener() {
                    @Override
                    public void onProgressChanged(SeekBar seekBar, int progress, boolean fromUser) {
                        if (Build.VERSION.SDK_INT < 29) {
                            return;
                        }
                        seekBar.setStateDescription("state description " + progress);
                    }

                    @Override
                    public void onStartTrackingTouch(SeekBar seekBar) {}

                    @Override
                    public void onStopTrackingTouch(SeekBar seekBar) {}
                });

        // TODO(sarakato): Set contents of webView element once b/150734712 has been resolved.
    }
}
