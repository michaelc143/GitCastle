package auth

import (
	"encoding/hex"
	"errors"
)

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

func hexEncode(data []byte) string {
	return hex.EncodeToString(data)
}
