package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-github/v38/github"
	"github.com/k1LoW/exec"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"gopkg.in/yaml.v2"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("! %+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root, err := getGHQRoot(ctx)
	if err != nil {
		return err
	}
	client, err := newClient(ctx)
	if err != nil {
		return err
	}
	authenUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("Users.Get: %w", err)
	}
	repos, err := listAllMatchedCode(ctx, client, fmt.Sprintf("user:%s path:.github/workflows", authenUser.GetLogin()))
	if err != nil {
		return err
	}
	if err := runAllActionLint(ctx, runtime.NumCPU(), root, repos); err != nil {
		return err
	}
	return nil
}

func getGHQRoot(ctx context.Context) (string, error) {
	cmd := (&exec.Exec{
		Signal:          os.Interrupt,
		KillAfterCancel: -1,
	}).CommandContext(ctx, "ghq", "root")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func runActionLint(ctx context.Context, repoDir string) error {
	stat, err := os.Stat(repoDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot stat: %w", err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not directory", repoDir)
	}
	cmd := (&exec.Exec{
		Signal:          os.Interrupt,
		KillAfterCancel: -1,
	}).CommandContext(ctx, "actionlint")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err == nil {
		return nil
	}
	log.Printf("actionlint(%s): %s", repoDir, out)
	return err
}

func runAllActionLint(ctx context.Context, maxWorkers int, ghqRoot string, repos []*github.Repository) error {
	sem := semaphore.NewWeighted(int64(maxWorkers))
	for _, repo := range repos {
		name := repo.GetFullName()
		repoDir := filepath.Join(ghqRoot, "github.com", name)
		if err := sem.Acquire(ctx, 1); err != nil {
			log.Printf("! cannot acquire semaphore: %s", err)
			break
		}
		go func(repoDir string) {
			defer sem.Release(1)
			if err := runActionLint(ctx, repoDir); err != nil {
				log.Printf("! repository=%s: %s", repoDir, err)
			}
		}(repoDir)
	}
	return nil
}

func listAllMatchedCode(ctx context.Context, client *github.Client, query string) ([]*github.Repository, error) {
	opts := &github.SearchOptions{}
	var repos []*github.Repository
	seen := map[string]bool{}
	for {
		searchResult, resp, err := client.Search.Code(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range searchResult.CodeResults {
			if !seen[r.Repository.GetFullName()] {
				repos = append(repos, r.Repository)
			}
			seen[r.Repository.GetFullName()] = true
		}
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
		opts.Page = resp.NextPage
	}
	return repos, nil
}

func newClient(ctx context.Context) (*github.Client, error) {
	cfg, err := getHubConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get hub config: %w", err)
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.dotcomToken()})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc), nil
}

func getHubConfig() (hubConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("os.UserHomeDir: %w", err)
	}
	p := filepath.Join(home, ".config", "hub")
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("os.Open(%s): %w", p, err)
	}
	defer f.Close()
	var c hubConfig
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("cannot decode YAML: %w", err)
	}
	return c, nil
}

type hostConfig map[string]string

func (c hostConfig) token() string {
	return c["oauth_token"]
}

type hubConfig map[string][]hostConfig

func (c hubConfig) dotcomToken() string {
	for _, host := range c["github.com"] {
		return host.token()
	}
	return ""
}
