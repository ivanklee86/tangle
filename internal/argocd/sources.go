package argocd

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// SourceKind says how ArgoCD fetches a source, which decides whether a git ref
// can apply to it at all.
type SourceKind string

const (
	// SourceKindGit is a git repo: plain manifests, a chart by path, a CMP, or
	// a `ref` source other sources read values from.
	SourceKindGit SourceKind = "git"
	// SourceKindHelm is a chart from a Helm repository. Its targetRevision is a
	// chart version, never a git ref.
	SourceKindHelm SourceKind = "helm"
	// SourceKindOCI is an artifact from an OCI registry. Its targetRevision is
	// a tag or digest in the registry, never a git ref.
	SourceKindOCI SourceKind = "oci"
)

// Source is one entry of an Application's sources, flattened to what Tangle
// needs to pick revisions and to show the caller what was compared.
type Source struct {
	// Position is ArgoCD's 1-based source position, as SourcePositions uses.
	Position       int64
	RepoURL        string
	Path           string
	Chart          string
	Ref            string
	TargetRevision string
	Kind           SourceKind
	// UsedBy lists the positions of the sources that read files through this
	// source's Ref (a Helm valueFiles entry like `$values/...`). Empty when
	// it has no Ref or nothing uses it.
	UsedBy []int64
}

// SourcesOf returns an Application's sources in ArgoCD's order. A single-source
// Application (spec.source) comes back as one source at position 1. An
// Application with neither returns nil.
func SourcesOf(app *v1alpha1.Application) []Source {
	if app == nil {
		return nil
	}

	var specSources []v1alpha1.ApplicationSource
	switch {
	case app.Spec.HasMultipleSources():
		specSources = app.Spec.Sources
	case app.Spec.Source != nil:
		specSources = []v1alpha1.ApplicationSource{*app.Spec.Source}
	default:
		return nil
	}

	users := refUsers(specSources)
	sources := make([]Source, 0, len(specSources))
	for i, specSource := range specSources {
		sources = append(sources, Source{
			Position:       int64(i + 1),
			RepoURL:        specSource.RepoURL,
			Path:           specSource.Path,
			Chart:          specSource.Chart,
			Ref:            specSource.Ref,
			TargetRevision: specSource.TargetRevision,
			Kind:           kindOf(specSource),
			UsedBy:         users[specSource.Ref],
		})
	}

	return sources
}

func kindOf(source v1alpha1.ApplicationSource) SourceKind {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.RepoURL)), "oci://") {
		return SourceKindOCI
	}

	if source.Chart != "" {
		return SourceKindHelm
	}

	return SourceKindGit
}

// scpLikeURL matches git's scp-style syntax, `user@host:path`, which has no
// scheme and so doesn't parse as a URL.
var scpLikeURL = regexp.MustCompile(`^(?:[^@/]+@)?([^/:]+):(.+)$`)

// portPrefix matches a port at the start of what follows `host:`.
var portPrefix = regexp.MustCompile(`^[0-9]+/`)

// NormalizeRepoURL reduces a git repo URL to `host/path`, so that one repo
// compares equal however it's spelled: https, ssh or scp-style, with or without
// a `.git` suffix, a trailing slash, credentials, a port or an upper-case host.
// The path keeps its case, because some git hosts treat it as significant.
func NormalizeRepoURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var host, path string
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Host == "" {
			return strings.ToLower(trimmed)
		}
		host = parsed.Hostname()
		path = parsed.Path
	} else if match := scpLikeURL.FindStringSubmatch(trimmed); match != nil {
		host = match[1]
		path = match[2]
		// `host:8443/org/repo`, typed without a scheme, means a port. Git's
		// scp syntax has no port, but it always carries `user@`, so only a
		// bare host is read this way: `git@host:2024/repo` keeps its path.
		if port := portPrefix.FindString(path); port != "" && !strings.Contains(trimmed, "@") {
			path = strings.TrimPrefix(path, port)
		}
	} else {
		// A bare `host/path`, as people tend to type into a form.
		host, path, _ = strings.Cut(trimmed, "/")
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")

	return strings.ToLower(host) + "/" + path
}

// refUsers returns, for each source with a `ref`, the positions of the other
// sources that read files through it: a Helm valueFiles entry or
// fileParameters path starting with `$<ref>/`, which is how ArgoCD lets one
// source read files from another.
func refUsers(specSources []v1alpha1.ApplicationSource) map[string][]int64 {
	users := map[string][]int64{}
	for _, source := range specSources {
		if source.Ref != "" {
			users[source.Ref] = nil
		}
	}

	for i, source := range specSources {
		if source.Helm == nil {
			continue
		}
		paths := append([]string{}, source.Helm.ValueFiles...)
		for _, parameter := range source.Helm.FileParameters {
			paths = append(paths, parameter.Path)
		}

		for ref := range users {
			for _, path := range paths {
				if strings.HasPrefix(path, "$"+ref+"/") {
					users[ref] = append(users[ref], int64(i+1))
					break
				}
			}
		}
	}

	return users
}
