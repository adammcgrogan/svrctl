// token generates the random control-socket auth tokens used to keep other
// local users from attaching to a server's console.
package process

import (
	"crypto/rand"
	"encoding/hex"
)

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
