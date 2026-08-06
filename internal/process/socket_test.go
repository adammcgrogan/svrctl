// socket_test covers the control-socket handshake's token check.
package process

import "testing"

func TestTokenMatches(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"

	if !tokenMatches(token, token) {
		t.Error("the correct token was rejected")
	}

	wrong := map[string]string{
		"empty":          "",
		"first byte":     "1123456789abcdef0123456789abcdef",
		"last byte":      "0123456789abcdef0123456789abcdee",
		"truncated":      token[:len(token)-1],
		"extra byte":     token + "0",
		"correct prefix": token[:8],
	}
	for label, guess := range wrong {
		if tokenMatches(guess, token) {
			t.Errorf("%s: %q was accepted", label, guess)
		}
	}
}
