package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/dtuit/ws/internal/git"
	"github.com/dtuit/ws/internal/manifest"
)

func resolveCommandRepos(m *manifest.Manifest, wsHome, filter string, includeWorktrees bool) ([]manifest.RepoInfo, error) {
	repos, err := resolveFilterRepos(m, wsHome, filter, false)
	if err != nil {
		return nil, err
	}
	if includeWorktrees {
		repos = expandSelectedReposToWorktrees(repos)
	}
	return repos, nil
}

func resolveContextRepos(m *manifest.Manifest, wsHome, filter string, includeWorktrees bool) ([]manifest.RepoInfo, error) {
	repos, err := resolveFilterRepos(m, wsHome, filter, true)
	if err != nil {
		return nil, err
	}
	if filter == "" || filter == "all" {
		repos = clonedRepos(repos)
	}
	if includeWorktrees {
		repos = expandSelectedReposToWorktrees(repos)
	}
	return repos, nil
}

func resolveFilterRepos(m *manifest.Manifest, wsHome, filter string, strict bool) ([]manifest.RepoInfo, error) {
	active := m.ActiveRepos()
	repoGroups := m.RepoGroups()

	if filter == manifest.EmptyFilter {
		return nil, nil
	}
	if filter == "" || filter == "all" {
		return m.AllRepos(wsHome), nil
	}

	seen := make(map[string]bool)
	result := make([]manifest.RepoInfo, 0, len(active))
	add := func(repo manifest.RepoInfo) {
		if seen[repo.Name] {
			return
		}
		seen[repo.Name] = true
		result = append(result, repo)
	}

	for _, token := range strings.Split(filter, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		if members, ok := m.Groups[token]; ok {
			for _, name := range members {
				cfg, ok := active[name]
				if !ok {
					continue
				}
				add(baseRepoInfo(m, wsHome, name, cfg, repoGroups[name]))
			}
			continue
		}

		if cfg, ok := active[token]; ok {
			add(baseRepoInfo(m, wsHome, token, cfg, repoGroups[token]))
			continue
		}

		repoName, selector, ok := splitWorktreeToken(token, active)
		if ok {
			if selector == "" {
				err := fmt.Errorf("worktree target %q is missing a worktree name", token)
				if strict {
					return nil, err
				}
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				continue
			}

			cfg := active[repoName]
			target, err := resolveExplicitWorktreeTarget(baseRepoInfo(m, wsHome, repoName, cfg, repoGroups[repoName]), selector)
			if err != nil {
				if strict {
					return nil, err
				}
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				continue
			}
			add(target)
			continue
		}

		err := fmt.Errorf("%q is not a known group, repo, or worktree target", token)
		if strict {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	return result, nil
}

func baseRepoInfo(m *manifest.Manifest, wsHome, name string, cfg manifest.RepoConfig, groups []string) manifest.RepoInfo {
	return manifest.RepoInfo{
		Name:   name,
		URL:    m.ResolveURL(name, cfg),
		Branch: m.ResolveBranch(cfg),
		Groups: groups,
		Path:   m.ResolvePath(wsHome, name, cfg),
	}
}

func expandSelectedReposToWorktrees(repos []manifest.RepoInfo) []manifest.RepoInfo {
	if len(repos) == 0 {
		return nil
	}

	baseRepos := make([]manifest.RepoInfo, 0, len(repos))
	for _, repo := range repos {
		if repo.Worktree != "" {
			continue
		}
		baseRepos = append(baseRepos, repo)
	}

	expandedByName := make(map[string][]manifest.RepoInfo, len(baseRepos))
	for _, set := range expandReposToWorktreeSets(baseRepos) {
		expandedByName[set.base.Name] = set.expanded
	}

	seen := make(map[string]bool)
	expanded := make([]manifest.RepoInfo, 0, len(repos))
	add := func(repo manifest.RepoInfo) {
		if seen[repo.Name] {
			return
		}
		seen[repo.Name] = true
		expanded = append(expanded, repo)
	}

	for _, repo := range repos {
		if repo.Worktree != "" {
			add(repo)
			continue
		}
		for _, target := range expandedByName[repo.Name] {
			add(target)
		}
	}

	return expanded
}

type worktreeExpansion struct {
	base     manifest.RepoInfo
	expanded []manifest.RepoInfo
}

func expandReposToWorktreeSets(repos []manifest.RepoInfo) []worktreeExpansion {
	sets := git.DiscoverWorktreesAll(repos, git.Workers(len(repos)))
	expanded := make([]worktreeExpansion, 0, len(sets))

	for _, set := range sets {
		if set.Err != nil || len(set.Worktrees) == 0 {
			expanded = append(expanded, worktreeExpansion{
				base:     set.Repo,
				expanded: []manifest.RepoInfo{set.Repo},
			})
			continue
		}

		reposForBase := make([]manifest.RepoInfo, 0, len(set.Worktrees))
		for _, target := range worktreeTargets(set.Repo, set.Worktrees) {
			worktreeName := ""
			if !target.Primary {
				worktreeName = worktreeDisplayName(set.Repo.Name, target.Name)
			}
			reposForBase = append(reposForBase, manifest.RepoInfo{
				Name:     target.Name,
				URL:      set.Repo.URL,
				Branch:   target.Branch,
				Groups:   set.Repo.Groups,
				Path:     target.Path,
				Worktree: worktreeName,
			})
		}
		expanded = append(expanded, worktreeExpansion{
			base:     set.Repo,
			expanded: reposForBase,
		})
	}

	return expanded
}
