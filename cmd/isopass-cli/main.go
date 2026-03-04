package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"isopass-client/internal/client"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "configure":
		cmdConfigure(os.Args[2:])
	case "login":
		cmdLogin(os.Args[2:])
	case "me":
		cmdMe(os.Args[2:])
	case "search":
		cmdSearch(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: isopass <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  configure  Save server URL and bearer token")
	fmt.Fprintln(os.Stderr, "  login      Open browser for SSO login")
	fmt.Fprintln(os.Stderr, "  me         Show client info")
	fmt.Fprintln(os.Stderr, "  search     Search secrets")
}

func cmdConfigure(args []string) {
	fs := flag.NewFlagSet("configure", flag.ExitOnError)
	server := fs.String("server", "", "Server URL (e.g. https://isopass.example.com)")
	token := fs.String("token", "", "Bearer token")
	tlsSkip := fs.Bool("tls-skip-verify", false, "Skip TLS certificate verification (for self-signed certs)")
	tlsCACert := fs.String("tls-ca-cert", "", "Path to CA certificate file (for self-signed certs)")
	fs.Parse(args)

	if *server == "" || *token == "" {
		fmt.Fprintln(os.Stderr, "Usage: isopass configure -server URL -token TOKEN [-tls-skip-verify] [-tls-ca-cert PATH]")
		os.Exit(1)
	}

	cfg := &client.Config{ServerURL: *server, BearerToken: *token, TLSSkipVerify: *tlsSkip, TLSCACert: *tlsCACert}
	if err := client.SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Configuration saved.")
}

func cmdMe(args []string) {
	fs := flag.NewFlagSet("me", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	c, err := client.NewFromConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	info, err := c.Me()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(info)
		return
	}

	fmt.Printf("Email: %s\n", info.Email)
	if len(info.Organizations) > 0 {
		fmt.Printf("Organizations: %s\n", strings.Join(info.Organizations, ", "))
	}
	if len(info.Scopes) > 0 {
		fmt.Printf("Scopes: %s\n", strings.Join(info.Scopes, ", "))
	}
}

func cmdSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	typeFilter := fs.String("type", "", "Filter by secret type")
	reveal := fs.Bool("reveal", false, "Show secret values")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	terms := strings.Join(fs.Args(), ",")

	c, err := client.NewFromConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	secrets, err := c.Search(200)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	secrets = client.FilterSecrets(secrets, terms, *typeFilter)

	if *jsonOut {
		if !*reveal {
			for i := range secrets {
				secrets[i].Value = ""
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(secrets)
		return
	}

	if len(secrets) == 0 {
		fmt.Println("No matches.")
		return
	}

	mask := "••••••••••••"
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ORG\tTYPE\tTAGS\tVALUE\n")
	for _, s := range secrets {
		tags := strings.Join(s.Tags, ", ")
		fields := client.ParseFields(s.Type, s.Value)
		var valueParts []string
		for _, f := range fields {
			display := f.Value
			if f.Secret && !*reveal {
				display = mask
			}
			if f.Label != "" && f.Label != "Value" {
				valueParts = append(valueParts, f.Label+": "+display)
			} else {
				valueParts = append(valueParts, display)
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Organization, s.Type, tags, strings.Join(valueParts, " | "))
	}
	w.Flush()
	fmt.Printf("\n%d result(s)\n", len(secrets))
}

func cmdLogin(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	server := fs.String("server", "", "Server URL (e.g. https://isopass.example.com)")
	fs.Parse(args)

	serverURL := *server
	if serverURL == "" {
		// Try loading from saved config.
		cfg, err := client.LoadConfig()
		if err == nil && cfg.ServerURL != "" {
			serverURL = cfg.ServerURL
		}
	}
	if serverURL == "" {
		fmt.Fprintln(os.Stderr, "Usage: isopass login -server URL")
		fmt.Fprintln(os.Stderr, "  or configure a server first: isopass configure -server URL -token TOKEN")
		os.Exit(1)
	}

	if !strings.HasPrefix(serverURL, "https://") {
		fmt.Fprintln(os.Stderr, "Error: server URL must use HTTPS")
		os.Exit(1)
	}

	// Build HTTP client with TLS config from saved config.
	httpClient := client.HTTPClientFromConfig()

	// Check if OIDC is enabled.
	resp, err := httpClient.Get(serverURL + "/api/auth/oidc/status")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	var result struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(body, &result); err != nil || !result.Enabled {
		fmt.Fprintln(os.Stderr, "SSO is not enabled on this server.")
		os.Exit(1)
	}

	url := serverURL + "/api/auth/oidc/authorize"
	fmt.Printf("Open this URL in your browser to sign in:\n\n  %s\n\n", url)
	fmt.Println("After authentication, copy the bearer token and run:")
	fmt.Println("  isopass configure -server URL -token TOKEN")
}
