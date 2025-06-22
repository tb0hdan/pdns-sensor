package clients

type Client interface {
	SubmitDomains(domains []string) error
}
