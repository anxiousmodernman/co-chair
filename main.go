package main

import (
	"crypto/tls"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/Rudd-O/curvetls"
	"github.com/anxiousmodernman/goth/gothic"
	"github.com/asdine/storm"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	// TODO look into newer versions of grpcweb and wsproxy. Have they merged?
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/johanbrandhorst/protobuf/wsproxy"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/anxiousmodernman/co-chair/backend"
	"github.com/anxiousmodernman/co-chair/config"
	"github.com/anxiousmodernman/co-chair/grpcclient"
	"github.com/anxiousmodernman/co-chair/proto/server"
	"github.com/dchest/uniuri"
)

// Version is our software version.
var Version = "0.1.0"

var (
	// Store is our sessions store.
	// TODO: this doesn't have to be a global
	Store  *sessions.CookieStore
	logger *logrus.Logger
)

func init() {
	secret := os.Getenv("COCHAIR_COOKIESTORE_SECRET")
	if secret == "" {
		secret = uniuri.NewLen(64)
	}
	Store = sessions.NewCookieStore([]byte(secret))
	logger = logrus.StandardLogger()
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
		DisableSorting:  true,
	})

	// set the grpc logger
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(logger.Out, logger.Out, logger.Out))

	// set up sessions store
	store := sessions.NewFilesystemStore(os.TempDir(), []byte(secret))
	store.MaxLength(math.MaxInt64)
	gothic.Store = store // TODO remove?

	// prevent error:
	// gob: type not registered for interface: map[string]interface {}
	var t map[string]interface{}
	gob.Register(t)
}

func main() {

	app := cli.NewApp()
	app.Version = Version

	// shared flags
	var (
		conf = cli.StringFlag{
			Name:  "conf",
			Usage: "path to config file",
		}
	)

	dbFlag := cli.StringFlag{
		Name:  "db",
		Usage: "path to boltdb file",
		Value: "co-chair.db",
	}

	apiCert := cli.StringFlag{
		Name:  "apiCert",
		Usage: "for grpc mgmt api: path to pem encoded tls certificate",
		Value: "./cert.pem",
	}

	apiClientValidation := cli.BoolTFlag{
		Name:  "apiClientValidation",
		Usage: "an api client's curvetls public keys must exist in database",
	}

	apiKey := cli.StringFlag{
		Name:  "apiKey",
		Usage: "for grpc mgmt api: path to pem encoded tls private key",
		Value: "./key.pem",
	}

	apiPort := cli.StringFlag{
		Name:  "apiPort",
		Usage: "port number for grpc mgmt api",
	}

	webCert := cli.StringFlag{
		Name:  "webUICert",
		Usage: "for web ui: path to pem encoded tls certificate",
		Value: "./cert.pem",
	}

	webDomain := cli.StringFlag{
		Name:   "webUIDomain",
		Usage:  "for web ui: our fully-qualified domain name",
		Value:  "localhost",
		EnvVar: "COCHAIR_WEBUI_DOMAIN",
	}

	webKey := cli.StringFlag{
		Name:  "webUIKey",
		Usage: "for web ui: path to pem encoded tls private key",
		Value: "./key.pem",
	}

	webPort := cli.StringFlag{
		Name:  "webUIPort",
		Usage: "port number for web ui",
		Value: "2016",
	}

	webAssetsPath := cli.StringFlag{
		Name:  "webAssetsPath",
		Usage: "serve given directory if provided, else use binary-embedded assets",
	}

	proxyCert := cli.StringFlag{
		Name:  "proxyCert",
		Usage: "for proxy: path to pem encoded tls certificate",
		Value: "./cert.pem",
	}

	proxyKey := cli.StringFlag{
		Name:  "proxyKey",
		Usage: "for proxy: path to pem encoded tls private key",
		Value: "./key.pem",
	}

	proxyPort := cli.StringFlag{
		Name:  "proxyPort",
		Usage: "port number for http proxy",
		Value: "8080",
	}

	proxyInsecurePort := cli.StringFlag{
		Name:  "proxyInsecurePort",
		Usage: "if provided, start a plaintext HTTP proxy on this port",
	}

	auth0ClientID := cli.StringFlag{
		Name:   "auth0ClientID",
		Usage:  "Auth0 Client ID for this co-chair instance",
		EnvVar: "COCHAIR_AUTH0_CLIENTID",
	}

	auth0Domain := cli.StringFlag{
		Name:   "auth0Domain",
		Usage:  "Auth0 Domain for this co-chair instance",
		EnvVar: "COCHAIR_AUTH0_DOMAIN",
	}
	auth0Secret := cli.StringFlag{
		Name:   "auth0Secret",
		Usage:  "Auth0 Secret",
		EnvVar: "COCHAIR_AUTH0_SECRET",
	}

	bypassAuth0 := cli.BoolFlag{
		Name:  "bypassAuth0",
		Usage: "totally bypass auth0; insecure development mode",
	}

	// client-only flags
	var (
		upstreamDomain = cli.StringFlag{
			Name:  "domain",
			Usage: "an upstream domain; pair with --\"ips\"",
		}

		upstreamIPs = cli.StringSliceFlag{
			Name:  "ips",
			Usage: "comma-separated list of the real host:port of an upstream; pair with --\"domain\"",
		}
	)
	app.Commands = []cli.Command{
		cli.Command{
			Name:  "example-config",
			Usage: "print an example co-chair TOML config to stdout",
			Action: func(_ *cli.Context) error {
				fmt.Println(config.ExampleConfig)
				return nil
			},
		},
		cli.Command{
			Name:  "gen-client-keys",
			Usage: "generate a (curvetls) client keypair and add public key to the keystore; co-chair cannot be running",
			Flags: []cli.Flag{conf, dbFlag},
			Action: func(ctx *cli.Context) error {
				// name is the identifier of our keypair
				name := ctx.Args().First()
				return genClientKeypair(name, ctx.String("db"))
			},
		},
		cli.Command{
			Name:  "put",
			Usage: "add an upstream to the proxy",
			Flags: []cli.Flag{conf, upstreamDomain, upstreamIPs},
			Action: func(ctx *cli.Context) error {
				clientConf, err := grpcclient.NewClientConfig(ctx.String("conf"))
				if err != nil {
					return err
				}
				c, err := grpcclient.NewCoChairClient(clientConf)
				if err != nil {
					return err
				}
				return c.Put(ctx.String("domain"), ctx.StringSlice("ips"))
			},
		},

		cli.Command{
			Name:  "serve",
			Usage: "run co-chair",
			Flags: []cli.Flag{dbFlag, apiCert, apiClientValidation, apiKey, apiPort,
				webCert, webDomain, webKey, webPort, webAssetsPath,
				proxyCert, proxyKey, proxyPort, proxyInsecurePort,
				auth0ClientID, auth0Domain, auth0Secret, bypassAuth0,
				conf},
			Action: func(ctx *cli.Context) error {
				conf, err := config.FromCLIOpts(ctx)
				if err != nil {
					return err
				}
				return serve(conf)
			},
		},
		cli.Command{
			Name:  "state",
			Usage: "report the proxy's upstream configuration",
			Flags: []cli.Flag{conf, upstreamDomain},
			Action: func(ctx *cli.Context) error {
				clientConf, err := grpcclient.NewClientConfig(ctx.String("conf"))
				if err != nil {
					return err
				}
				c, err := grpcclient.NewCoChairClient(clientConf)
				if err != nil {
					return err
				}
				return c.State(ctx.String("domain"))
			},
		},
		cli.Command{
			Name:  "systemd-install",
			Usage: "installs a systemd unit file and config directory",
			Flags: []cli.Flag{conf},
			Action: func(ctx *cli.Context) error {
				conf, err := config.FromCLIOpts(ctx)
				if err != nil {
					return err
				}
				return config.SystemDInstall(conf)
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal(err)
	}
}

func serve(conf config.Config) error {

	// TODO: construct storm.DB here and pass to constructors instead of
	// grabbing the field off the Proxy.

	// NewProxy gives us a Proxy, our concrete implementation of the
	// interface generated by the grpc protobuf compiler.
	px, err := backend.NewProxy(conf.DBPath)
	if err != nil {
		log.Fatalf("proxy init: %v", err)
	}

	// KeyStore is an interface, so it is nil if unset.
	var keystore curvetls.KeyStore
	if conf.APIClientValidation {
		keystore = &backend.StormKeystore{DB: px.DB}
	}

	// curvetls transport security
	serverPub, serverPriv, err := backend.RetrieveServerKeys(px.DB)
	if err != nil {
		return fmt.Errorf("could not retrieve server's keypair %v", err)
	}
	creds := curvetls.NewGRPCServerCredentials(serverPub, serverPriv, keystore)

	grpcOnlyServer := grpc.NewServer(grpc.Creds(creds))
	server.RegisterProxyServer(grpcOnlyServer, px)

	// gRPC over websockets management API
	gs := grpc.NewServer()
	server.RegisterProxyServer(gs, px)
	wrappedServer := grpcweb.WrapServer(gs)

	webTLScreds, err := credentials.NewClientTLSFromFile(conf.WebUICert, "")
	if err != nil {
		return errors.New("missing web ui tls cert")
	}

	wsproxy := wsproxy.WrapServer(
		wrappedServer,
		wsproxy.WithLogger(logger),
		wsproxy.WithTransportCredentials(webTLScreds))

	// Note: routes are evaluated in the order they're defined.
	p := mux.NewRouter()

	// Set up our authentication handler, with optional bypass
	authHandler := IsAuthenticated
	if conf.BypassAuth0 {
		logger.Info("insecure configuration: bypassing auth0 protection for webUI")
		authHandler = func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			next(w, r)
		}
	}

	p.Handle("/login", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(loginLink)),
	)).Methods("GET")

	p.Handle("/auth/{provider}/callback", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(oauthCallbackHandler)),
	)).Methods("GET")

	p.Handle("/auth/{provider}", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(loginHandler)),
	)).Methods("GET")

	p.Handle("/logout/{provider}", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(logoutHandler)),
	)).Methods("GET")

	allowCORS := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "X-Grpc-Web", "content-type", "x-grpc-web"}),
	)

	// All websockets requests are POSTs to some {method} on this route.
	p.Handle("/web.Proxy/{method}", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(authHandler),
		negroni.Wrap(allowCORS(websocketsProxy(wsproxy))),
	)).Methods("POST", "OPTIONS")

	// Dynamically construct static handlers
	// serve frontend from embedded binary assets

	if conf.WebAssetsPath != "" {
		logger.Info("WebAssetsPath: ", conf.WebAssetsPath)
		p.MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			match, _ := regexp.MatchString("/.*", r.URL.Path)
			return match
		}).Handler(
			negroni.New(
				setConf(conf),
				negroni.HandlerFunc(withLog),
				negroni.HandlerFunc(authHandler),
				negroni.Wrap(staticFromDiskHandler(conf.WebAssetsPath)),
			)).Methods("GET")
	} else {
		p.MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			match, _ := regexp.MatchString("/.*", r.URL.Path)
			return match
		}).Handler(
			negroni.New(
				setConf(conf),
				negroni.HandlerFunc(withLog),
				negroni.HandlerFunc(authHandler),
				negroni.Wrap(http.HandlerFunc(staticHandler)),
			)).Methods("GET")
	}

	// else we serve the "static" folder
	if true {
		walker := func(r *mux.Route, rt *mux.Router, anc []*mux.Route) error {
			rx, _ := r.GetPathRegexp()
			tmpl, _ := r.GetPathTemplate()
			logger.Infof("route regex: %v", rx)
			logger.Infof("route template: %v", tmpl)
			return nil
		}
		p.Walk(mux.WalkFunc(walker))
	}

	// Web server for our Vecty/GopherJS management UI
	httpsSrv := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%s", conf.WebUIPort),
		// TODO fix me, make me configurable based on local/prod builds
		Handler:           handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(p),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		TLSConfig: &tls.Config{
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
		},
	}

	grpcAPI := make(chan error)
	proxy := make(chan error)
	pureGRPC := make(chan error)

	// Only start an external API listener if we're validating client keys.
	if conf.APIClientValidation {
		logger.Infof("starting external gRPC listener on port %s", conf.APIPort)
		lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", conf.APIPort))
		if err != nil {
			return fmt.Errorf("listener error: %v", err)
		}
		go func() { pureGRPC <- grpcOnlyServer.Serve(lis) }()
	}

	// Start our web UI listener
	go func() {
		logger.Info("Serving Web UI on https://" + httpsSrv.Addr)
		grpcAPI <- httpsSrv.ListenAndServeTLS(conf.WebUICert, conf.WebUIKey)
	}()

	fwdr, err := backend.NewTCPForwarder(
		backend.WithDB(px.DB),
		backend.WithAddr(fmt.Sprintf("0.0.0.0:%s", conf.ProxyPort)),
		backend.WithLogger(logger),
	)
	if err != nil {
		return err
	}
	// TCPForwarder routine
	if err := fwdr.Start(); err != nil {
		return fmt.Errorf("start TCPFowarder: %v", err)
	}

	if conf.ProxyInsecurePort != "" {
		// should we even support this?
	}

	for {
		select {
		case err := <-grpcAPI:
			return err
		case err := <-proxy:
			return err
		case err := <-pureGRPC:
			return err
		}
	}
}

func genClientKeypair(name, dbPath string) error {
	// NOTE this function directly accesses the database. It's a command line
	// feature, and assumes local access to the database file.

	db, err := storm.Open(dbPath)
	if err != nil {
		return err
	}

	serverPub, _, err := backend.RetrieveServerKeys(db)
	if err != nil {
		return err
	}

	priv, pub, err := curvetls.GenKeyPair()
	if err != nil {
		return err
	}
	clientKP := backend.KeyPair{Name: name, Pub: pub.String()}
	if err := db.Save(&clientKP); err != nil {
		return err
	}

	msg := `The following are new curveTLS public and private keys.\n`
	fmt.Println(msg)

	fmt.Println("client public key (new):\t", pub)
	fmt.Println("client private key (new):\t", priv)
	fmt.Println("server public key:\t", serverPub)

	return nil
}
