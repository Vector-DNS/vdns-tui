package validate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

const maxDomainLength = 253

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`)

var blockedTLDs = map[string]bool{
	"local":    true,
	"internal": true,
	"lan":      true,
	"arpa":     true,
	"localhost": true,
	"home":     true,
	"corp":     true,
	"test":     true,
	"invalid":  true,
	"example":  true,
}

var blockedHostnames = map[string]bool{
	"localhost":    true,
	"wpad":         true,
	"autoconfig":   true,
	"autodiscover": true,
}

// Package-level IDNA profile - immutable, safe to reuse.
var idnaProfile = idna.New(
	idna.MapForLookup(),
	idna.ValidateForRegistration(),
)

type Result struct {
	Original string
	ASCII    string
	Unicode  string
	IsIDN    bool
}

// DisplayName returns the domain for display, showing both unicode and ASCII for IDN domains.
func (r *Result) DisplayName() string {
	if r.IsIDN {
		return fmt.Sprintf("%s (%s)", r.Unicode, r.ASCII)
	}
	return r.ASCII
}

// Domain validates and normalizes a domain name.
func Domain(input string) (*Result, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	if !utf8.ValidString(input) {
		return nil, fmt.Errorf("domain contains invalid UTF-8")
	}

	// Single pass: check for control chars and detect non-ASCII.
	isASCII := true
	for i, r := range input {
		if r == 0 {
			return nil, fmt.Errorf("domain contains null byte at position %d", i)
		}
		if r < 32 {
			return nil, fmt.Errorf("domain contains control character at position %d", i)
		}
		if r > 127 {
			isASCII = false
		}
	}

	domain := strings.TrimSuffix(strings.TrimSpace(input), ".")
	domain = strings.ToLower(domain)

	if len(domain) == 0 {
		return nil, fmt.Errorf("domain cannot be empty after normalization")
	}

	if len(domain) > maxDomainLength {
		return nil, fmt.Errorf("domain exceeds maximum length of %d characters", maxDomainLength)
	}

	result := &Result{Original: input}

	if isASCII {
		result.ASCII = domain
		result.Unicode = domain
	} else {
		ascii, err := idnaProfile.ToASCII(domain)
		if err != nil {
			return nil, fmt.Errorf("invalid internationalized domain name: %w", err)
		}
		unicode, err := idnaProfile.ToUnicode(domain)
		if err != nil {
			unicode = domain
		}
		result.ASCII = ascii
		result.Unicode = unicode
		result.IsIDN = true
	}

	if !domainRegex.MatchString(result.ASCII) {
		return nil, fmt.Errorf("invalid domain format: %s", result.ASCII)
	}

	// Extract TLD and check against blocked list (O(1) lookup).
	lastDot := strings.LastIndex(result.ASCII, ".")
	if lastDot >= 0 {
		tld := result.ASCII[lastDot+1:]
		if blockedTLDs[tld] {
			return nil, fmt.Errorf("blocked TLD: .%s", tld)
		}
	}

	parts := strings.SplitN(result.ASCII, ".", 2)
	if len(parts) > 0 && blockedHostnames[parts[0]] {
		return nil, fmt.Errorf("blocked hostname: %s", parts[0])
	}

	return result, nil
}

// Domains validates multiple domain names. Does not stop on the first error.
func Domains(inputs []string) ([]*Result, []error) {
	var results []*Result
	var errs []error
	for _, input := range inputs {
		r, err := Domain(input)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", input, err))
		} else {
			results = append(results, r)
		}
	}
	return results, errs
}
