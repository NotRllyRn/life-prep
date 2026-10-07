package main

import (
	"testing"
	"time"
)

func TestDispatchWakeCoalescesAndDoesNotBlock(t *testing.T) {
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			wakeDispatch(wake)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mailbox ingestion blocked")
	}
	if len(wake) != 1 {
		t.Fatal("wakeups not coalesced")
	}
	<-wake
	wakeDispatch(wake)
	if len(wake) != 1 {
		t.Fatal("new work did not wake dispatcher")
	}
}
