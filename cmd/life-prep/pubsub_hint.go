package main

import "encoding/json"

// notificationEmail ignores the unused history hint, which Gmail may encode
// as either a JSON number or a string. The authoritative cursor is in SQLite.
func notificationEmail(b []byte) (string, error) {
	var hint struct{ EmailAddress string }
	err := json.Unmarshal(b, &hint)
	return hint.EmailAddress, err
}
