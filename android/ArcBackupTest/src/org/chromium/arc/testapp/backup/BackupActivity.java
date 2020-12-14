/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.backup;

import android.content.Context;
import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.widget.EditText;
import android.widget.TextView;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.util.Date;

public class BackupActivity extends Activity {
    private static final long EXPIRATION_TIME_MS = 60 * 60 * 1000; // 1 hour.

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_backup);
    }

    public void saveButton(View view) {
        EditText editText = (EditText) findViewById(R.id.edit_message);
        String fileName = editText.getText().toString();
        FileOutputStream outputStream;

        try {
            outputStream = openFileOutput(fileName, Context.MODE_PRIVATE);
            outputStream.close();
        } catch (IOException e) {
            e.printStackTrace();
        }
    }

    public void loadButton(View view) {
        EditText editText = (EditText) findViewById(R.id.edit_message);
        String fileName = editText.getText().toString();

        TextView textView = (TextView) findViewById(R.id.file_content);
        FileInputStream inputStream;
        try {
            inputStream = openFileInput(fileName);
            textView.setText("Success");
            inputStream.close();
        } catch (IOException e) {
            textView.setText("Fail");
        }
    }

    public void clearButton(View view) {
        EditText editText = (EditText) findViewById(R.id.edit_message);
        String fileName = editText.getText().toString();
        deleteFile(fileName);
        clearOldFiles();
    }

    private void clearOldFiles() {
        // Clear old files that are created before EXPIRATION_TIME_MS ago.
        for (File file : getFilesDir().listFiles()) {
            try {
                Date now = new Date();
                long createdTime = Long.parseLong(file.getName());
                if (now.getTime() - createdTime > EXPIRATION_TIME_MS) {
                    deleteFile(file.getName());
                }
            } catch (NumberFormatException e) {
                deleteFile(file.getName());
            }
        }
    }
}
