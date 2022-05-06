package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v44/github"
	flag "github.com/spf13/pflag"
)

var flags = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

var privateKeyPath string
var appID, installationID int64

var method string

func init() {
	flags.SetOutput(os.Stdout)
	flags.SortFlags = false

	flags.StringVarP(&privateKeyPath, "private-key", "p", "", "path to the GitHub App private key")
	flags.Int64VarP(&appID, "app-id", "a", 0, "GitHub App ID")
	flags.Int64VarP(&installationID, "installation-id", "i", 0, "GitHub App installation ID")

	flags.StringVarP(&method, "method", "X", "GET", "HTTP method")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	validate()

	client := newClient()

	endpoint := flags.Arg(0)
	if endpoint == "" {
		endpoint = "/"
	}

	req, err := client.NewRequest(method, endpoint, nil)
	if err != nil {
		log.Fatal(err)
	}

	// pass through directly; client.NewRequest takes a value to marshal
	req.Body = os.Stdin

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	_, err = client.Do(ctx, req, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

func newClient() *github.Client {
	tr := http.DefaultTransport

	itr, err := ghinstallation.NewKeyFromFile(tr, appID, installationID, privateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	return github.NewClient(&http.Client{Transport: itr})
}

func validate() {
	if privateKeyPath == "" {
		log.Fatal("missing --private-key/-p")
	}

	if appID == 0 {
		log.Fatal("missing --app-id/-a")
	}

	if installationID == 0 {
		log.Fatal("missing --installation-id/-i")
	}
}
