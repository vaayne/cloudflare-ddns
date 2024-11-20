package config

import (
	"context"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const dockerLabelKey = "cf_ddns_domain"

// ReadDomains reads an environment variable as a comma-separated list of domains.
func ReadDomains(ppfmt pp.PP, key string, field *[]domain.Domain) bool {
	if list, ok := domainexp.ParseList(ppfmt, key, Getenv(key)); ok {
		*field = list
		return true
	}
	return false
}

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list []domain.Domain) []domain.Domain {
	domain.SortDomains(list)
	return slices.Compact(list)
}

// ReadDomainsFromDockerLabels reads domains from Docker container labels
func ReadDomainsFromDockerLabels(ppfmt pp.PP, domains *[]domain.Domain) bool {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		ppfmt.Infof(pp.EmojiError, "Failed to create Docker client: %v", err)
		return false
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		ppfmt.Infof(pp.EmojiError, "Failed to list Docker containers: %v", err)
		return false
	}

	for _, container := range containers {
		if domain, ok := container.Labels[dockerLabelKey]; ok {
			if parsed, ok := domainexp.ParseList(ppfmt, "Docker label cf_ddns_domain", domain); ok {
				*domains = append(*domains, parsed...)
			} else {
				ppfmt.Infof(pp.EmojiError, "Failed to parse domains in container %s", container.Labels["cf_ddns_domain"])
			}
		}
	}

	return true
}

// ReadDomainMap reads environment variables DOMAINS, IP4_DOMAINS, and IP6_DOMAINS,
// as well as Docker container labels, and consolidates the domains into a map.
func ReadDomainMap(ppfmt pp.PP, field *map[ipnet.Type][]domain.Domain) bool {
	var domains, ip4Domains, ip6Domains []domain.Domain

	// Read domains from environment variables
	ReadDomains(ppfmt, "DOMAINS", &domains)
	// Read domains from Docker labels
	ReadDomainsFromDockerLabels(ppfmt, &domains)
	ReadDomains(ppfmt, "IP4_DOMAINS", &ip4Domains)
	ReadDomains(ppfmt, "IP6_DOMAINS", &ip6Domains)

	ip4Domains = deduplicate(append(ip4Domains, domains...))
	ip6Domains = deduplicate(append(ip6Domains, domains...))

	if len(ip4Domains) == 0 && len(ip6Domains) == 0 {
		return false
	}

	*field = map[ipnet.Type][]domain.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}

	return true
}
