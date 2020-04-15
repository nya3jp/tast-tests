/*
 * Copyright (C) 2016 The Android Open Source Project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package org.chromium.arc.testapp.notification;

import android.app.Activity;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.support.annotation.NonNull;
import android.support.v4.app.NotificationCompat;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.util.Log;

public class NotificationActivity extends Activity implements View.OnClickListener {
    private static final String TAG = "ArcTest.NotificationActivity";

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.notification_activity);

        ((Button) findViewById(R.id.send_button)).setOnClickListener(this);
        ((Button) findViewById(R.id.remove_button)).setOnClickListener(this);
    }

    @Override public void onClick(View v) {
        switch (v.getId()) {
            case R.id.send_button:
                sendNotification(
                        Integer.parseInt(getEditTextValue(R.id.notification_id)),
                        getEditTextValue(R.id.notification_title),
                        getEditTextValue(R.id.notification_text));
                break;
            case R.id.remove_button:
                removeNotification(
                        Integer.parseInt(getEditTextValue(R.id.notification_id)));
                break;
        }
    }

    @Override
    public void onResume() {
        super.onResume();

        Intent intent = getIntent();
        if (intent != null) {
            int id = intent.getIntExtra("id", 0);
            String title = intent.getStringExtra("title");
            String text = intent.getStringExtra("text");

            if (title != null && text != null) {
                Log.i(TAG, "Sending notification, title: " + title + ", text: " + text +
                        ", id: " + id);
                sendNotification(id, title, text);
            } else {
                Log.e(TAG, "Invalid argument, title: " + title + ", text: " + text);
            }
        }
    }

    private @NonNull String getEditTextValue(int view_id) {
        return ((EditText) findViewById(view_id)).getText().toString();
    }

    private void sendNotification(int id, @NonNull String title, @NonNull String text) {
        NotificationCompat.Builder builder = new NotificationCompat.Builder(this)
                .setSmallIcon(R.drawable.ic_adb_black_24dp)
                .setContentTitle(title)
                .setContentText(text);
        NotificationManager notificationManager =
                (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        notificationManager.notify(id, builder.build());
    }

    private void removeNotification(int id) {
        NotificationManager notificationManager =
                (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        notificationManager.cancel(id);
    }
}
