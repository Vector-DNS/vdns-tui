package dns

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	mdns "github.com/miekg/dns"
)

// DefaultResolvers is the base set of public DNS resolvers for propagation checks.
var DefaultResolvers = []Resolver{
	{Address: "8.8.8.8:53", Name: "Google DNS"},
	{Address: "8.8.4.4:53", Name: "Google DNS Secondary"},
	{Address: "1.1.1.1:53", Name: "Cloudflare DNS"},
	{Address: "1.0.0.1:53", Name: "Cloudflare DNS Secondary"},
	{Address: "9.9.9.9:53", Name: "Quad9"},
	{Address: "149.112.112.112:53", Name: "Quad9 Secondary"},
	{Address: "208.67.222.222:53", Name: "OpenDNS"},
	{Address: "208.67.220.220:53", Name: "OpenDNS Secondary"},
	{Address: "76.76.2.0:53", Name: "Control D"},
	{Address: "76.76.10.0:53", Name: "Control D Secondary"},
	{Address: "94.140.14.14:53", Name: "AdGuard DNS"},
	{Address: "94.140.15.15:53", Name: "AdGuard DNS Secondary"},
	{Address: "185.228.168.9:53", Name: "CleanBrowsing"},
	{Address: "185.228.169.9:53", Name: "CleanBrowsing Secondary"},
	{Address: "4.2.2.1:53", Name: "Level3"},
}

// DefaultResolverIPs returns just the IP addresses from DefaultResolvers.
func DefaultResolverIPs() []string {
	ips := make([]string, len(DefaultResolvers))
	for i, r := range DefaultResolvers {
		host, _, _ := net.SplitHostPort(r.Address)
		ips[i] = host
	}
	return ips
}

// SupportedTypes is the canonical list of DNS record types the CLI supports.
var SupportedTypes = []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT", "SOA", "CAA", "SRV"}

type Resolver struct {
	Address string
	Name    string
}

type Record struct {
	Type     string
	Value    string
	TTL      uint32
	Priority *uint16
}

type LookupResult struct {
	Domain      string
	Records     map[string][]Record
	Resolver    string
	QueryTimeMs int64
	Timestamp   time.Time
}

type PropagationResult struct {
	Resolver Resolver
	Values   []string
	TTL      *uint32
	Success  bool
	Error    string
}

var (
	cachedResolver     string
	cachedResolverOnce sync.Once
)

// Lookup performs a DNS lookup using the system resolver.
// Queries all supported types if none are specified.
func Lookup(domain string, types []string) (*LookupResult, error) {
	if len(types) == 0 {
		types = SupportedTypes
	}

	resolver := getSystemResolver()
	result := &LookupResult{
		Domain:    domain,
		Records:   make(map[string][]Record),
		Resolver:  resolver,
		Timestamp: time.Now(),
	}

	start := time.Now()

	type typeResult struct {
		rtype   string
		records []Record
	}

	ch := make(chan typeResult, len(types))
	var wg sync.WaitGroup

	c := new(mdns.Client)
	c.Timeout = 5 * time.Second

	for _, t := range types {
		upper := strings.ToUpper(t)
		qtype, ok := mdns.StringToType[upper]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(rtype string, qt uint16) {
			defer wg.Done()

			m := new(mdns.Msg)
			m.SetQuestion(mdns.Fqdn(domain), qt)
			m.RecursionDesired = true

			r, _, err := c.Exchange(m, resolver)
			if err != nil || r == nil || r.Rcode != mdns.RcodeSuccess {
				return
			}

			records := parseAnswers(r.Answer, rtype)
			if len(records) > 0 {
				ch <- typeResult{rtype: rtype, records: records}
			}
		}(upper, qtype)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for tr := range ch {
		result.Records[tr.rtype] = tr.records
	}

	result.QueryTimeMs = time.Since(start).Milliseconds()
	return result, nil
}

// CheckPropagation queries multiple resolvers concurrently for a domain and record type.
func CheckPropagation(domain string, recordType string, resolvers []Resolver) ([]PropagationResult, bool) {
	if len(resolvers) == 0 {
		resolvers = DefaultResolvers
	}
	if recordType == "" {
		recordType = "A"
	}

	qtype, ok := mdns.StringToType[strings.ToUpper(recordType)]
	if !ok {
		qtype = mdns.TypeA
	}

	results := make([]PropagationResult, len(resolvers))
	var wg sync.WaitGroup

	c := new(mdns.Client)
	c.Timeout = 5 * time.Second

	for i, res := range resolvers {
		results[i] = PropagationResult{Resolver: res}

		wg.Add(1)
		go func(idx int, resolver Resolver) {
			defer wg.Done()

			m := new(mdns.Msg)
			m.SetQuestion(mdns.Fqdn(domain), qtype)
			m.RecursionDesired = true

			r, _, err := c.Exchange(m, resolver.Address)
			if err != nil {
				results[idx].Error = err.Error()
				return
			}

			if r == nil {
				results[idx].Error = "no response"
				return
			}

			if r.Rcode != mdns.RcodeSuccess {
				results[idx].Error = fmt.Sprintf("DNS error: %s", mdns.RcodeToString[r.Rcode])
				return
			}

			results[idx].Success = true
			for _, ans := range r.Answer {
				results[idx].Values = append(results[idx].Values, extractValue(ans))
				if results[idx].TTL == nil {
					ttl := ans.Header().Ttl
					results[idx].TTL = &ttl
				}
			}
		}(i, res)
	}

	wg.Wait()
	return results, checkConsistency(results)
}

// BenchmarkResult holds timing data for a single resolver.
type BenchmarkResult struct {
	Resolver  Resolver
	LatencyMs int64
	Success   bool
	Error     string
}

// Benchmark measures response time for each resolver by querying a domain.
func Benchmark(domain string, resolvers []Resolver) []BenchmarkResult {
	if len(resolvers) == 0 {
		resolvers = DefaultResolvers
	}

	results := make([]BenchmarkResult, len(resolvers))
	var wg sync.WaitGroup

	c := new(mdns.Client)
	c.Timeout = 5 * time.Second

	for i, res := range resolvers {
		results[i] = BenchmarkResult{Resolver: res}

		wg.Add(1)
		go func(idx int, resolver Resolver) {
			defer wg.Done()

			m := new(mdns.Msg)
			m.SetQuestion(mdns.Fqdn(domain), mdns.TypeA)
			m.RecursionDesired = true

			start := time.Now()
			r, _, err := c.Exchange(m, resolver.Address)
			results[idx].LatencyMs = time.Since(start).Milliseconds()

			if err != nil {
				results[idx].Error = err.Error()
				return
			}
			if r == nil || r.Rcode != mdns.RcodeSuccess {
				results[idx].Error = "no valid response"
				return
			}
			results[idx].Success = true
		}(i, res)
	}

	wg.Wait()
	return results
}

var isDefaultResolver bool

// GetSystemResolver returns the detected system resolver address.
func GetSystemResolver() string {
	return getSystemResolver()
}

// IsDefaultResolver returns true if the system resolver could not be detected
// and the fallback (Google DNS) is being used instead.
func IsDefaultResolver() bool {
	getSystemResolver()
	return isDefaultResolver
}

func getSystemResolver() string {
	cachedResolverOnce.Do(func() {
		config, err := mdns.ClientConfigFromFile("/etc/resolv.conf")
		if err == nil && len(config.Servers) > 0 {
			cachedResolver = net.JoinHostPort(config.Servers[0], config.Port)
		} else {
			cachedResolver = "8.8.8.8:53"
			isDefaultResolver = true
		}
	})
	return cachedResolver
}

func parseAnswers(answers []mdns.RR, queryType string) []Record {
	var records []Record
	for _, ans := range answers {
		rec := Record{
			Type:  queryType,
			Value: extractValue(ans),
			TTL:   ans.Header().Ttl,
		}
		if mx, ok := ans.(*mdns.MX); ok {
			prio := mx.Preference
			rec.Priority = &prio
		}
		records = append(records, rec)
	}
	return records
}

func extractValue(rr mdns.RR) string {
	switch v := rr.(type) {
	case *mdns.A:
		return v.A.String()
	case *mdns.AAAA:
		return v.AAAA.String()
	case *mdns.CNAME:
		return strings.TrimSuffix(v.Target, ".")
	case *mdns.MX:
		return strings.TrimSuffix(v.Mx, ".")
	case *mdns.NS:
		return strings.TrimSuffix(v.Ns, ".")
	case *mdns.TXT:
		return strings.Join(v.Txt, " ")
	case *mdns.SOA:
		return fmt.Sprintf("%s %s %d %d %d %d %d",
			strings.TrimSuffix(v.Ns, "."),
			strings.TrimSuffix(v.Mbox, "."),
			v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)
	case *mdns.CAA:
		return fmt.Sprintf("%d %s \"%s\"", v.Flag, v.Tag, v.Value)
	case *mdns.SRV:
		return fmt.Sprintf("%d %d %d %s",
			v.Priority, v.Weight, v.Port,
			strings.TrimSuffix(v.Target, "."))
	default:
		return rr.String()
	}
}

func checkConsistency(results []PropagationResult) bool {
	var reference []string
	for _, r := range results {
		if !r.Success {
			continue
		}
		if reference == nil {
			reference = slices.Clone(r.Values)
			slices.Sort(reference)
			continue
		}
		b := slices.Clone(r.Values)
		slices.Sort(b)
		if !slices.Equal(reference, b) {
			return false
		}
	}
	return true
}
