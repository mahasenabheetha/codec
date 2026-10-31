package encode

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxUUIDs bounds a bulk run.
const MaxUUIDs = 1000

// UUIDOptions says which UUIDs to make: version 4 (random, the default),
// 7 (time-ordered, random tail), or 5 and 3 (the same every time for a
// namespace and name, hashed with SHA-1 or MD5).
type UUIDOptions struct {
	Version   int    `json:"version"`
	Namespace string `json:"namespace"` // dns, url, oid, x500, or a UUID
	Name      string `json:"name"`
}

// namespaces are RFC 9562's predefined ones.
var namespaces = map[string]uuid.UUID{"dns": uuid.NameSpaceDNS, "url": uuid.NameSpaceURL, "oid": uuid.NameSpaceOID, "x500": uuid.NameSpaceX500}

// NewUUIDs makes count UUIDs (1 to MaxUUIDs). Name-based versions give
// the same UUID each time, so they come back once.
func NewUUIDs(o UUIDOptions, count int) ([]string, error) {
	count = min(max(count, 1), MaxUUIDs)
	switch o.Version {
	case 0, 4:
		out := make([]string, count)
		for i := range out {
			out[i] = UUID()
		}
		return out, nil
	case 7:
		out := make([]string, count)
		for i := range out {
			u, err := uuid.NewV7() // monotonic within a millisecond
			if err != nil {
				return nil, err
			}
			out[i] = u.String()
		}
		return out, nil
	case 3, 5:
		ns, err := namespaceOf(o.Namespace)
		if err != nil {
			return nil, err
		}
		if o.Version == 5 {
			return []string{uuid.NewSHA1(ns, []byte(o.Name)).String()}, nil
		}
		return []string{uuid.NewMD5(ns, []byte(o.Name)).String()}, nil
	}
	return nil, fmt.Errorf("version is 4, 7, 5 or 3, not %d", o.Version)
}

func namespaceOf(s string) (uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return uuid.NameSpaceDNS, nil
	}
	if ns, ok := namespaces[strings.ToLower(s)]; ok {
		return ns, nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("namespace is dns, url, oid, x500 or a UUID")
	}
	return u, nil
}

// UUIDInfo is what a UUID says about itself.
type UUIDInfo struct {
	UUID    string `json:"uuid"` // canonical: lowercase with hyphens
	Version int    `json:"version"`
	// Kind is what the version means: random, time-ordered, name-based…
	Kind    string `json:"kind"`
	Variant string `json:"variant"`
	// Time is when a version 1, 6 or 7 UUID was made (RFC 3339, UTC).
	Time string `json:"time,omitempty"`
	Nil  bool   `json:"nil,omitempty"` // all zeros
	Max  bool   `json:"max,omitempty"` // all ones
}

var versionKinds = map[int]string{
	1: "time-based (Gregorian time and node)",
	2: "DCE security",
	3: "name-based, MD5",
	4: "random",
	5: "name-based, SHA-1",
	6: "time-based, reordered for sorting",
	7: "time-ordered (Unix milliseconds, random tail)",
	8: "custom",
}

// ParseUUID reads a UUID written any common way: with or without
// hyphens, in braces, upper case, or as urn:uuid:.
func ParseUUID(s string) (UUIDInfo, error) {
	u, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return UUIDInfo{}, fmt.Errorf("not a UUID: 32 hex digits, usually 8-4-4-4-12")
	}
	info := UUIDInfo{UUID: u.String(), Version: int(u.Version()), Nil: u == uuid.Nil, Max: u == uuid.Max}
	info.Kind = versionKinds[info.Version]
	switch {
	case info.Nil:
		info.Kind, info.Version = "the nil UUID", 0
	case info.Max:
		info.Kind, info.Version = "the max UUID", 0
	case info.Kind == "":
		info.Kind = "unknown version"
	}
	switch u.Variant() {
	case uuid.RFC4122:
		info.Variant = "RFC 9562"
	case uuid.Reserved:
		info.Variant = "reserved (NCS)"
	case uuid.Microsoft:
		info.Variant = "Microsoft"
	default:
		info.Variant = "future"
	}
	if u.Variant() == uuid.RFC4122 {
		switch info.Version {
		case 1, 6, 7:
			sec, nsec := u.Time().UnixTime()
			info.Time = time.Unix(sec, nsec).UTC().Format(time.RFC3339Nano)
		}
	}
	return info, nil
}
