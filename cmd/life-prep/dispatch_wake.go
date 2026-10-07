package main

// wakeDispatch never blocks mailbox ingestion behind an agent request.
func wakeDispatch(wake chan<- struct{}) {
	select {
	case wake <- struct{}{}:
	default:
	}
}
