package main

import "testing"

func TestPubsubNumericHistoryHint(t *testing.T) {
	for _, payload := range []string{`{"emailAddress":"synthetic@example.invalid","historyId":123456}`, `{"emailAddress":"synthetic@example.invalid","historyId":"123456"}`} {
		email, err := notificationEmail([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		if email != "synthetic@example.invalid" {
			t.Fatal("email routing lost")
		}
	}
	if _, err := notificationEmail([]byte(`{"emailAddress":123}`)); err == nil {
		t.Fatal("invalid mailbox accepted")
	}
	if _, err := notificationEmail([]byte(`bad json`)); err == nil {
		t.Fatal("invalid notification accepted")
	}
}
