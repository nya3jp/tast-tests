/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.imeblocking;

import android.app.Activity;
import android.app.AlertDialog;
import android.os.Bundle;
import android.text.InputType;
import android.view.View;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.EditText;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        Button button = findViewById(R.id.button);
        button.setOnClickListener(v -> { openDialog(v); });

        Button button2 = findViewById(R.id.dialog_with_text_field);
        button2.setOnClickListener(v -> { openDialogWithTextField(v); });

        Button button3 = findViewById(R.id.open_normal_dialog_above);
        button3.setOnClickListener(v -> { openDialogWithTextField(v); openDialog(v); });

        Button button4 = findViewById(R.id.open_text_field_dialog_above);
        button4.setOnClickListener(v -> { openDialog(v); openDialogWithTextField(v); });
    }

    private void openDialog(View v) {
        AlertDialog.Builder builder = new AlertDialog.Builder(this);
        builder.setMessage(android.R.string.unknownName)
                .setPositiveButton(android.R.string.ok, null)
                .setNegativeButton(android.R.string.cancel, null);
        AlertDialog dialog = builder.create();
        dialog.create();
        dialog.getWindow()
                .clearFlags(
                        WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE
                                | WindowManager.LayoutParams.FLAG_ALT_FOCUSABLE_IM);
        dialog.getWindow()
                .setFlags(
                        WindowManager.LayoutParams.FLAG_ALT_FOCUSABLE_IM,
                        WindowManager.LayoutParams.FLAG_ALT_FOCUSABLE_IM);
        dialog.show();
    }

    private void openDialogWithTextField(View v) {
        final EditText input = findViewById(R.id.text_in_dialog);

        AlertDialog.Builder builder = new AlertDialog.Builder(this);
        builder.setMessage(android.R.string.unknownName)
            .setPositiveButton(android.R.string.ok, null)
            .setNegativeButton(android.R.string.cancel, null);
        builder.setView(input);

        AlertDialog dialog = builder.create();
        dialog.create();
        dialog.show();
    }
}
