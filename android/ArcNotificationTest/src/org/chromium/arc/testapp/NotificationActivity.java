/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.notification;

import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.CheckBox;
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
        if (intent == null) {
            return;
        }

        int id = intent.getIntExtra("id", 0);
        String title = intent.getStringExtra("title");
        String text = intent.getStringExtra("text");

        if (title == null || text == null) {
            Log.e(TAG, "Invalid argument, title: " + title + ", text: " + text);
            return;
        }
        Log.i(TAG, "Sending notification, title: " + title + ", text: " + text +
                ", id: " + id);
        sendNotification(id, title, text);
    }

    private String getEditTextValue(int view_id) {
        return ((EditText) findViewById(view_id)).getText().toString();
    }

    private void sendNotification(int id, String title, String text) {
        Notification.Builder builder = new Notification.Builder(this);
        NotificationManager notificationManager =
                (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        boolean high_priority = ((CheckBox) findViewById(R.id.check_high_priority)).isChecked();
        if (high_priority) {
          // Create a high priority channel for notification to be sent on.
          String channel_id = "ArcNotificationTest";
          NotificationChannel channel = new NotificationChannel(channel_id, "high_priority_channel",
                NotificationManager.IMPORTANCE_HIGH);
          builder = new Notification.Builder(this, channel_id);
          notificationManager.createNotificationChannel(channel);
        }

        builder.setSmallIcon(R.drawable.ic_adb_black_24dp)
        .setContentTitle(title)
        .setContentText(text);

        notificationManager.notify(id, builder.build());
    }

    private void removeNotification(int id) {
        NotificationManager notificationManager =
                (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        notificationManager.cancel(id);
    }
}
