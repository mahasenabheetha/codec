package encode

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// bcrypt costs offered; each step doubles the time (14 is about a
// second), and ingress controllers check the hash on every request.
const (
	MinCost     = 4
	MaxCost     = 14
	DefaultCost = 10
)

// Htpasswd makes a "user:hash" line with a bcrypt hash, as
// `htpasswd -nbB` prints it ($2y$), for nginx and Traefik basic auth.
func Htpasswd(user, password string, cost int) (string, error) {
	if user == "" {
		return "", errors.New("user name is empty")
	}
	if strings.ContainsAny(user, ":\n\r") {
		return "", errors.New("a user name can't contain \":\" or a line break")
	}
	if password == "" {
		return "", errors.New("password is empty")
	}
	if cost < MinCost || cost > MaxCost {
		return "", fmt.Errorf("cost must be %d to %d", MinCost, MaxCost)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "", errors.New("bcrypt uses at most 72 bytes of a password")
	}
	if err != nil {
		return "", err
	}
	// Go writes $2a$; Apache's htpasswd writes $2y$ for the same hash.
	return user + ":$2y$" + string(h[4:]), nil
}

// HtpasswdCheck reports whether password matches a "user:hash" line
// (or a bare hash). Only bcrypt hashes can be checked.
func HtpasswdCheck(line, password string) (bool, error) {
	line = strings.TrimSpace(line)
	hash := line
	if i := strings.LastIndexByte(line, ':'); i >= 0 && !strings.HasPrefix(line, "$") {
		hash = line[i+1:]
	}
	switch {
	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
	case strings.HasPrefix(hash, "$apr1$"), strings.HasPrefix(hash, "{SHA}"), strings.HasPrefix(hash, "$1$"):
		return false, errors.New("only bcrypt hashes ($2y$) can be checked; this one is an older, weaker kind")
	default:
		return false, errors.New("not a bcrypt hash ($2y$…)")
	}
	// The cost is in the hash, and bcrypt allows up to 31: about two days
	// of CPU for one check. Refuse what codec wouldn't make.
	if cost, err := bcrypt.Cost([]byte(hash)); err != nil {
		return false, fmt.Errorf("unreadable bcrypt hash: %w", err)
	} else if cost > MaxCost {
		return false, fmt.Errorf("this hash has cost %d; codec checks hashes up to cost %d (each step doubles the time)", cost, MaxCost)
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("unreadable bcrypt hash: %w", err)
	}
	return true, nil
}
