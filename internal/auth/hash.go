package auth

import "crypto/sha256"

// sha256Sum hashes a session token before it touches the database, so a
// database leak does not expose usable credentials.
func sha256Sum(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hexEncode(sum[:])
}
