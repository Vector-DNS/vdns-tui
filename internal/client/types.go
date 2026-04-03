package client

import "time"

type DNSLookupRequest struct {
	Domain string   `json:"domain"`
	Types  []string `json:"types,omitempty"`
}

type DNSRecord struct {
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
}

type DNSLookupResponse struct {
	Domain      string                `json:"domain"`
	Records     map[string][]DNSRecord `json:"records"`
	Resolver    string                `json:"resolver"`
	QueryTimeMs int                   `json:"query_time_ms"`
	DNSSEC      bool                  `json:"dnssec"`
	Timestamp   string                `json:"timestamp"`
}

type PropagationRequest struct {
	Domain string `json:"domain"`
	Type   string `json:"type,omitempty"`
}

type ResolverResult struct {
	Resolver string   `json:"resolver"`
	Name     string   `json:"name"`
	Values   []string `json:"values"`
	TTL      *int     `json:"ttl,omitempty"`
	Success  bool     `json:"success"`
	Error    string   `json:"error,omitempty"`
}

type PropagationResponse struct {
	Domain     string           `json:"domain"`
	Type       string           `json:"type"`
	Results    []ResolverResult `json:"results"`
	Consistent bool            `json:"consistent"`
	QueriedAt  string           `json:"queried_at"`
}

type WhoisRequest struct {
	Domain string `json:"domain"`
}

type WhoisResult struct {
	DomainName  string   `json:"domain_name"`
	Registrar   *string  `json:"registrar"`
	CreatedDate *string  `json:"created_date"`
	UpdatedDate *string  `json:"updated_date"`
	ExpiresDate *string  `json:"expires_date"`
	Nameservers []string `json:"nameservers"`
	Status      []string `json:"status"`
	DNSSEC      *string  `json:"dnssec"`
}

type WhoisResponse struct {
	Domain    string      `json:"domain"`
	Result    WhoisResult `json:"result"`
	QueriedAt string      `json:"queried_at"`
}

type AvailabilityRequest struct {
	Domain string `json:"domain"`
}

type AvailabilityResult struct {
	DomainName string   `json:"domain_name"`
	Available  bool     `json:"available"`
	RDAPStatus []string `json:"rdap_status"`
}

type AvailabilityResponse struct {
	Result    AvailabilityResult `json:"result"`
	QueriedAt string             `json:"queried_at"`
}

type SSLResponse struct {
	Domain      string     `json:"domain"`
	Issuer      *string    `json:"issuer"`
	Subject     *string    `json:"subject"`
	ValidFrom   *time.Time `json:"valid_from"`
	ExpiresAt   *time.Time `json:"expires_at"`
	ChainLength int        `json:"chain_length"`
	Protocol    *string    `json:"protocol"`
	IsValid     bool       `json:"is_valid"`
	Error       *string    `json:"error"`
	CheckedAt   string     `json:"checked_at"`
}

type RateLimit struct {
	Limit      int
	Remaining  int
	RetryAfter int
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}
