package lib

import (
	"context"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type GithubRepository github.Repository

type Updateable interface {
	updateToMatch(comparable Comparable) bool
}

type Comparable interface {

	isSameAs(comparable Comparable) bool

	comparables() []Comparable
}

func (a GithubRepository) isSameAs(b GithubRepository) bool {
	return &a.Name == &b.Name
}

func (a GithubRepository) comparables() []Comparable {
	return []Comparable{}
}


func exec() {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "eeb81ff15b288fa009e6e46a1b84ca5e040b0752"},
	)
	ctx := context.Background()
	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	original, _, _ := githubClient.Repositories.Get(ctx, "coreos", "issue-sync")
	fork, _, _ := githubClient.Repositories.Get(ctx, "crfeliz", "issue-sync")

	for pair := range []Comparable{GithubRepository(*original), GithubRepository(*fork)} {

	}

	GithubRepository(*original).isSameAs(GithubRepository(*fork))
}


func main() {

}
