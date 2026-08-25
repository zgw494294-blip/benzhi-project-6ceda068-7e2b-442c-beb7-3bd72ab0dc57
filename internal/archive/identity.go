package archive

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

type IDGenerator interface {
	New(prefix string) string
}

type RandomIDGenerator struct {
	counter atomic.Uint64
}

func (g *RandomIDGenerator) New(prefix string) string {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err == nil {
		return prefix + "_" + hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("%s_%x_%x", prefix, time.Now().UnixNano(), g.counter.Add(1))
}

func stableTaskID(idempotencyKey string) string {
	sum := sha256.Sum256([]byte("archive-task-v1\n" + idempotencyKey))
	return "task_" + hex.EncodeToString(sum[:12])
}
