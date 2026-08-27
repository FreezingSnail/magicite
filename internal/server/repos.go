package server

import "github.com/FreezingSnail/magicite/internal/repo"

// RepoView is the repository record exposed through the daemon socket API.
type RepoView struct {
	Name   string `json:"name"`
	Root   string `json:"root"`
	Prefix string `json:"prefix"`
	Branch string `json:"branch"`
}

func repoViews(list []repo.Repo) []RepoView {
	views := make([]RepoView, 0, len(list))
	for _, record := range list {
		views = append(views, RepoView{
			Name:   record.Name,
			Root:   record.Root,
			Prefix: record.Prefix,
			Branch: record.Branch,
		})
	}
	return views
}

func repoNames(list []repo.Repo) []string {
	names := make([]string, 0, len(list))
	for _, record := range list {
		names = append(names, record.Name)
	}
	return names
}
