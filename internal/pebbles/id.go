package pebbles

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

const defaultIssueIDSuffixLength = 3

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

// GenerateIssueID derives a deterministic ID with the default suffix length.
func GenerateIssueID(prefix, title, timestamp, host string) string {
	hash := issueIDHash(prefix, title, timestamp, host)
	return issueIDFromHash(prefix, hash, defaultIssueIDSuffixLength)
}

// GenerateUniqueIssueID derives a deterministic ID and expands the suffix on collision.
func GenerateUniqueIssueID(prefix, title, timestamp, host string, exists func(string) (bool, error)) (string, error) {
	hash := issueIDHash(prefix, title, timestamp, host)
	for length := defaultIssueIDSuffixLength; length <= len(hash); length++ {
		issueID := issueIDFromHash(prefix, hash, length)
		// Check for collisions against existing issue IDs.
		inUse, err := exists(issueID)
		if err != nil {
			return "", err
		}
		// Return the first available suffix length.
		if !inUse {
			return issueID, nil
		}
	}
	return "", fmt.Errorf("issue id collision for %s", prefix)
}

// issueIDHash builds the full hex hash used for issue IDs.
func issueIDHash(prefix, title, timestamp, host string) string {
	hash := sha256.Sum256([]byte(prefix + ":" + timestamp + ":" + title + ":" + host))
	return hex.EncodeToString(hash[:])
}

// issueIDFromHash formats an issue ID with a specific hash length.
func issueIDFromHash(prefix, hash string, length int) string {
	return fmt.Sprintf("%s-%s", prefix, hash[:length])
}
