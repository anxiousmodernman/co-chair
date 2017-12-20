package config

import "github.com/urfave/cli"

// FromCLIOpts builds a Config from command line options and env vars.
// The urfave/cli Context gives us access to both on program start, and also
// allows us to set defaults.
func FromCLIOpts(ctx *cli.Context) Config {
	var c Config
	c.DBPath = ctx.String("db")
	c.APICert = ctx.String("apiCert")
	c.APIKey = ctx.String("apiKey")
	c.WebUICert = ctx.String("webUICert")
	c.WebUIKey = ctx.String("webUIKey")
	c.ProxyCert = ctx.String("proxyCert")
	c.ProxyKey = ctx.String("proxyKey")
	c.Auth0ClientID = ctx.String("auth0ClientID")
	c.Auth0Secret = ctx.String("auth0Secret")
	c.BypassAuth0 = ctx.BoolT("bypassAuth0")
	return c
}

// The Config struct. Contains config values suitable for multiple processes we
// spin off, including the pure gRPC management API, the http(s) listener for
// our GopherJS Web UI (including websockets), and the listener for our proxy
// forwarding implementation.
type Config struct {
	// Filepath to the boltdb file.
	DBPath string

	// APICert and APIKey are paths to PEM-encoded TLS assets
	// for our pure gRPC api
	APICert, APIKey string

	// WebUICert and WebUIKey are paths to PEM-encoded TLS assets
	// for our GopherJS-over-websockets UI. This UI is wrapped with
	// auth0 handlers.
	WebUICert, WebUIKey string

	// ProxyCert and  are paths to PEM-encoded TLS assets
	// for our GopherJS-over-websockets UI. This UI is wrapped with
	// auth0 handlers.
	ProxyCert, ProxyKey string

	// Auth0 config values
	Auth0ClientID, Auth0Secret string
	BypassAuth0                bool
}
