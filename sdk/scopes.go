package sdk

import "fmt"

type Scope string

const (
	ScopeOrgMembersRead   Scope = "org:members:read"
	ScopeOrgMembersWrite  Scope = "org:members:write"
	ScopeOrgDomainsRead   Scope = "org:domains:read"
	ScopeOrgDomainsWrite  Scope = "org:domains:write"
	ScopeOrgPoliciesRead  Scope = "org:policies:read"
	ScopeOrgPoliciesWrite Scope = "org:policies:write"

	ScopeVideosRead  Scope = "media:videos:read"
	ScopeVideosWrite Scope = "media:videos:write"
	ScopeImagesRead  Scope = "media:images:read"
	ScopeImagesWrite Scope = "media:images:write"
	ScopeFilesRead   Scope = "media:files:read"
	ScopeFilesWrite  Scope = "media:files:write"
	ScopeFeedsRead   Scope = "media:feeds:read"
	ScopeFeedsWrite  Scope = "media:feeds:write"
	ScopePostsRead   Scope = "media:posts:read"
	ScopePostsWrite  Scope = "media:posts:write"

	ScopeProjectsRead  Scope = "project:projects:read"
	ScopeProjectsWrite Scope = "project:projects:write"

	ScopeAnalyticsEventsRead Scope = "analytics:events:read"

	ScopeBillingRead  Scope = "billing:subscription:read"
	ScopeBillingWrite Scope = "billing:subscription:write"

	ScopeAPIKeysRead     Scope = "credentials:apikeys:read"
	ScopeAPIKeysWrite    Scope = "credentials:apikeys:write"
	ScopeClientAuthRead  Scope = "credentials:clientauth:read"
	ScopeClientAuthWrite Scope = "credentials:clientauth:write"
)

var AllScopes = []Scope{
	ScopeOrgMembersRead, ScopeOrgMembersWrite,
	ScopeOrgDomainsRead, ScopeOrgDomainsWrite,
	ScopeOrgPoliciesRead, ScopeOrgPoliciesWrite,
	ScopeVideosRead, ScopeVideosWrite,
	ScopeImagesRead, ScopeImagesWrite,
	ScopeFilesRead, ScopeFilesWrite,
	ScopeFeedsRead, ScopeFeedsWrite,
	ScopePostsRead, ScopePostsWrite,
	ScopeProjectsRead, ScopeProjectsWrite,
	ScopeAnalyticsEventsRead,
	ScopeBillingRead, ScopeBillingWrite,
	ScopeAPIKeysRead, ScopeAPIKeysWrite,
	ScopeClientAuthRead, ScopeClientAuthWrite,
}

var MediaReadScopes = []Scope{ScopeImagesRead, ScopeVideosRead, ScopeFeedsRead, ScopeFilesRead}

var MediaWriteScopes = []Scope{ScopeImagesWrite, ScopeVideosWrite, ScopeFeedsWrite, ScopeFilesWrite}

var knownScopes = func() map[Scope]struct{} {
	m := make(map[Scope]struct{}, len(AllScopes))
	for _, s := range AllScopes {
		m[s] = struct{}{}
	}
	return m
}()

func (s Scope) Valid() bool {
	_, ok := knownScopes[s]
	return ok
}

func (s Scope) String() string { return string(s) }

func validateScopes(scopes []Scope) error {
	for _, s := range scopes {
		if !s.Valid() {
			return fmt.Errorf("unknown scope %q", s)
		}
	}
	return nil
}
