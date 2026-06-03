package shared

import (
	"crypto/rand"
	"encoding/hex"
)

// NewLifeID 生成生命体本地 ID。
// Phase 0 形式：local-<16hex>。Phase 1 会被 DID 取代。
func NewLifeID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "local-" + hex.EncodeToString(b)
}
