package pebbles

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// NowTimestamp returns the current UTC time in RFC3339Nano.
func NowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// HostLabel returns a stable host identifier for ID generation.
func HostLabel() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "unknown"
	}
	return host
}

// GenerateIssueID derives an ID from prefix, timestamp, title, and host.
func GenerateIssueID(prefix, title, timestamp, host string) string {
	hash := sha256.Sum256([]byte(prefix + ":" + timestamp + ":" + title + ":" + host))
	suffix := hex.EncodeToString(hash[:4])
	return fmt.Sprintf("%s-%s", prefix, suffix)
}
