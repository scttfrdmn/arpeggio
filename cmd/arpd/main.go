// Command arpd is the arpeggio control-plane API.
//
// It runs as a single Lambda behind an API Gateway HTTP API, and as a plain
// HTTP server locally. It is deliberately NOT VPC-attached: it reaches STS,
// EC2, DynamoDB, and Globus over public endpoints, which is what keeps the
// control plane free at rest (CLAUDE.md golden rule 3).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/scttfrdmn/arpeggio/internal/auth"
	appcfg "github.com/scttfrdmn/arpeggio/internal/config"
	"github.com/scttfrdmn/arpeggio/internal/httpx"
	"github.com/scttfrdmn/arpeggio/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "arpd: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg, err := appcfg.Load()
	if err != nil {
		return err
	}

	authn, err := buildAuthenticator(ctx, cfg)
	if err != nil {
		return err
	}

	handler := httpx.NewServer(cfg, authn).Routes()

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(newLambdaAdapter(handler))
		return nil
	}

	addr := envOr("ARP_ADDR", ":8080")
	fmt.Printf("arpd listening on %s\n", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// buildAuthenticator wires the real Globus + DynamoDB stack, or the in-memory
// fake stack under ARP_FAKE_GLOBUS. The fake path touches neither AWS config
// nor OIDC discovery, so it runs with no network and no Globus client.
func buildAuthenticator(ctx context.Context, cfg *appcfg.Config) (*auth.Authenticator, error) {
	if cfg.Fake {
		fmt.Fprintln(os.Stderr, "arpd: ARP_FAKE_GLOBUS set — using in-memory fake Globus; do not use in a deployed environment")
		return auth.NewFakeAuthenticator(cfg, store.NewMemory()), nil
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	table := store.New(dynamodb.NewFromConfig(awsCfg), cfg.TableName)
	dir := auth.NewHTTPDirectory(cfg.GlobusClientID, cfg.GlobusClientSecret)

	authn, err := auth.NewAuthenticator(ctx, cfg, dir, table)
	if err != nil {
		return nil, err
	}
	return authn, nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
