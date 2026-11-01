package account

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrInvalidAccountID is returned when an account ID is malformed or invalid.
var ErrInvalidAccountID = errors.New("account: invalid account id")

const (
	// ULIDLength is the exact length of a canonical ULID string.
	ULIDLength = 26

	// crockfordBase32 is the lowercase Crockford Base32 alphabet.
	crockfordBase32 = "0123456789abcdefghjkmnpqrstvwxyz"
)

var (
	// decTable maps ASCII byte to 5-bit Base32 value. 0xFF indicates invalid.
	decTable [256]byte
	initOnce sync.Once
)

func initDecTable() {
	for i := range decTable {
		decTable[i] = 0xFF
	}
	for i := 0; i < len(crockfordBase32); i++ {
		decTable[crockfordBase32[i]] = byte(i)
	}
}

// encodeULID encodes 16 raw bytes into a 26-character lowercase ULID string.
func encodeULID(b [16]byte) string {
	var dst [26]byte

	// Byte 0 -> 2 chars
	dst[0] = crockfordBase32[(b[0]&0xE0)>>5]
	dst[1] = crockfordBase32[b[0]&0x1F]

	// Bytes 1..5 -> 8 chars
	encode5Bytes(dst[2:10], b[1:6])

	// Bytes 6..10 -> 8 chars
	encode5Bytes(dst[10:18], b[6:11])

	// Bytes 11..15 -> 8 chars
	encode5Bytes(dst[18:26], b[11:16])

	return string(dst[:])
}

func encode5Bytes(dst []byte, src []byte) {
	b0, b1, b2, b3, b4 := src[0], src[1], src[2], src[3], src[4]
	dst[0] = crockfordBase32[(b0&0xF8)>>3]
	dst[1] = crockfordBase32[((b0&0x07)<<2)|((b1&0xC0)>>6)]
	dst[2] = crockfordBase32[(b1&0x3E)>>1]
	dst[3] = crockfordBase32[((b1&0x01)<<4)|((b2&0xF0)>>4)]
	dst[4] = crockfordBase32[((b2&0x0F)<<1)|((b3&0x80)>>7)]
	dst[5] = crockfordBase32[(b3&0x7C)>>2]
	dst[6] = crockfordBase32[((b3&0x03)<<3)|((b4&0xE0)>>5)]
	dst[7] = crockfordBase32[b4&0x1F]
}

// NewULID generates a new 26-character lowercase ULID.
// It combines a 48-bit millisecond timestamp with 80 bits of cryptographic randomness.
func NewULID() (string, error) {
	return NewULIDAt(time.Now())
}

// NewULIDAt generates a ULID for a specific time.
func NewULIDAt(t time.Time) (string, error) {
	var raw [16]byte

	// 48-bit millisecond timestamp
	ms := uint64(t.UnixMilli())
	if ms > 0x0000FFFFFFFFFFFF {
		return "", fmt.Errorf("%w: timestamp overflow", ErrInvalidAccountID)
	}

	raw[0] = byte(ms >> 40)
	raw[1] = byte(ms >> 32)
	raw[2] = byte(ms >> 24)
	raw[3] = byte(ms >> 16)
	raw[4] = byte(ms >> 8)
	raw[5] = byte(ms)

	// 80-bit cryptographic entropy
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", fmt.Errorf("account: entropy read failed: %w", err)
	}

	return encodeULID(raw), nil
}

// ParseULID validates that raw is a canonical 26-character lowercase ULID.
func ParseULID(raw string) (string, error) {
	initOnce.Do(initDecTable)

	if len(raw) != ULIDLength {
		return "", fmt.Errorf("%w: length must be %d, got %d", ErrInvalidAccountID, ULIDLength, len(raw))
	}

	// Strictly require lowercase
	if raw != strings.ToLower(raw) {
		return "", fmt.Errorf("%w: must be lowercase", ErrInvalidAccountID)
	}

	// First character can only use 3 bits (0..7) to avoid 128-bit overflow
	if raw[0] > '7' {
		return "", fmt.Errorf("%w: first character %c exceeds 128-bit range", ErrInvalidAccountID, raw[0])
	}

	for i := 0; i < len(raw); i++ {
		if decTable[raw[i]] == 0xFF {
			return "", fmt.Errorf("%w: invalid Crockford character %q", ErrInvalidAccountID, raw[i])
		}
	}

	return raw, nil
}

// TimestampFromULID extracts the time.Time timestamp from a valid ULID.
func TimestampFromULID(ulidStr string) (time.Time, error) {
	id, err := ParseULID(ulidStr)
	if err != nil {
		return time.Time{}, err
	}

	initOnce.Do(initDecTable)
	// Decode top 10 characters (48 bits = 6 bytes)
	// First 2 chars -> byte 0
	c0 := decTable[id[0]]
	c1 := decTable[id[1]]
	b0 := (c0 << 5) | c1

	// Next 8 chars -> bytes 1..5
	c2 := decTable[id[2]]
	c3 := decTable[id[3]]
	c4 := decTable[id[4]]
	c5 := decTable[id[5]]
	c6 := decTable[id[6]]
	c7 := decTable[id[7]]
	c8 := decTable[id[8]]
	c9 := decTable[id[9]]

	b1 := (c2 << 3) | (c3 >> 2)
	b2 := ((c3 & 0x03) << 6) | (c4 << 1) | (c5 >> 4)
	b3 := ((c5 & 0x0F) << 4) | (c6 >> 1)
	b4 := ((c6 & 0x01) << 7) | (c7 << 2) | (c8 >> 3)
	b5 := ((c8 & 0x07) << 5) | c9

	var msBytes [8]byte
	msBytes[2] = b0
	msBytes[3] = b1
	msBytes[4] = b2
	msBytes[5] = b3
	msBytes[6] = b4
	msBytes[7] = b5

	ms := int64(binary.BigEndian.Uint64(msBytes[:]))
	return time.UnixMilli(ms), nil
}
