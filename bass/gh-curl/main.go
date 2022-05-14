package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v44/github"
	flag "github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

var flags = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

var privateKeyPath string
var appID, installationID int64

var tokenPath string

var method string

var trace bool

func init() {
	flags.SetOutput(os.Stdout)
	flags.SortFlags = false

	flags.StringVar(&privateKeyPath, "app-private-key", "", "path to a GitHub App private key to use for minting tokens")
	flags.Int64Var(&appID, "app-id", 0, "GitHub App ID")
	flags.Int64Var(&installationID, "installation-id", 0, "GitHub App installation ID")

	flags.StringVar(&tokenPath, "token", os.Getenv("GITHUB_TOKEN"), "path to a GitHub API token")

	flags.BoolVar(&trace, "trace", false, "dump the full HTTP response")

	flags.StringVarP(&method, "method", "X", "GET", "HTTP method")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	client, err := newClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	endpoint := flags.Arg(0)
	if endpoint == "" {
		endpoint = "/"
	}

	req, err := client.NewRequest(method, endpoint, nil)
	if err != nil {
		log.Fatal(err)
	}

	// pass through directly; client.NewRequest takes a value to marshal
	if method != "GET" {
		req.Header.Add("Content-Type", "application/json")
		req.Body = os.Stdin
	}

	req.Header.Add("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(ctx, req, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}

	if trace {
		fmt.Fprintln(os.Stderr)
		resp.Write(os.Stderr)
	}
}

func newClient(ctx context.Context) (*github.Client, error) {
	tr := http.DefaultTransport

	var httpClient *http.Client
	if privateKeyPath != "" && appID != 0 && installationID != 0 {
		itr, err := ghinstallation.NewKeyFromFile(tr, appID, installationID, privateKeyPath)
		if err != nil {
			return nil, err
		}

		httpClient = &http.Client{Transport: itr}
	} else if tokenPath != "" {
		tok, err := os.ReadFile(tokenPath)
		if err != nil {
			return nil, err
		}

		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(tok)},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	} else {
		return nil, fmt.Errorf("must specify --token or --app-private-key,--app-id,--installation-id")
	}

	return github.NewClient(httpClient), nil
}
