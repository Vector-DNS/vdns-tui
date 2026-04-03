package views

import (
	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
)

// Propagation check result (local).
type LocalPropagationResultMsg struct {
	Results    []dns.PropagationResult
	Consistent bool
	Err        error
}

// Propagation check result (remote).
type RemotePropagationResultMsg struct {
	Result *client.PropagationResponse
	Err    error
}

// Benchmark result.
type BenchmarkResultMsg struct {
	Results []dns.BenchmarkResult
	Err     error
}

// WHOIS result (remote only).
type WhoisResultMsg struct {
	Result *client.WhoisResponse
	Err    error
}

// Availability result (remote only).
type AvailabilityResultMsg struct {
	Result *client.AvailabilityResponse
	Err    error
}

// SSL result (remote only).
type SSLResultMsg struct {
	Result *client.SSLResponse
	Err    error
}
