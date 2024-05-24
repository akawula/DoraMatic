package repositories

import "github.com/shurcooL/githubv4"

type Repo struct {
	Name            githubv4.String
	PrimaryLanguage struct {
		Name githubv4.String
	}
	Owner struct {
		Login githubv4.String
	}
}
