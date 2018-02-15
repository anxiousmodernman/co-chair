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
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/Rudd-O/curvetls"
	"github.com/anxiousmodernman/goth/gothic"
	"github.com/asdine/storm"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/johanbrandhorst/protobuf/wsproxy"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/anxiousmodernman/co-chair/backend"
	"github.com/anxiousmodernman/co-chair/config"
	"github.com/anxiousmodernman/co-chair/grpcclient"
	"github.com/anxiousmodernman/co-chair/proto/server"
)

// Version is our software version.
var Version = "0.1.0"

var (
	// Store is our sessions store.
	Store *sessions.CookieStore
)

var logger *logrus.Logger

func init() {
	// TODO make this more secrety
	Store = sessions.NewCookieStore([]byte("something-very-secret"))
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
	store := sessions.NewFilesystemStore(os.TempDir(), []byte("secret-here?"))
	store.MaxLength(math.MaxInt64)
	gothic.Store = store

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
		Value: "1917",
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
				webCert, webDomain, webKey, webPort, proxyCert, proxyKey, proxyPort,
				proxyInsecurePort, auth0ClientID, auth0Domain, auth0Secret,
				bypassAuth0, conf},
			Action: func(ctx *cli.Context) error {
				conf, err := config.FromCLIOpts(ctx)
				if err != nil {
					return err
				}
				return run(conf)
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
			Usage: "installs a unit file and config directory",
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
		log.Fatal(err)
	}
}

func run(conf config.Config) error {

	// NewProxy gives us a Proxy, our concrete implementation of the
	// interface generated by the grpc protobuf compiler.
	px, err := backend.NewProxy(conf.DBPath)
	if err != nil {
		log.Fatalf("proxy init: %v", err)
	}

	// KeyStore is an interface, so it is nil if unset.
	var keystore curvetls.KeyStore
	if conf.APIClientValidation {
		// We can share the db pointer, it's cool.
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
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", conf.APIPort))
	if err != nil {
		return fmt.Errorf("listener error: %v", err)
	}

	// gRPC over websockets management API
	gs := grpc.NewServer()
	server.RegisterProxyServer(gs, px)
	wrappedServer := grpcweb.WrapServer(gs)

	webTLS, err := credentials.NewClientTLSFromFile(conf.WebUICert, "")
	if err != nil {
		return errors.New("Failed to get local server client credentials, did you run `make generate_cert`?")
	}

	wsproxy := wsproxy.WrapServer(
		wrappedServer,
		wsproxy.WithLogger(logger),
		wsproxy.WithTransportCredentials(webTLS))

	// Note: routes are evaluated in the order they're defined.
	p := mux.NewRouter()

	authHandler := IsAuthenticated
	if conf.BypassAuth0 {
		logger.Debug("bypassing auth")
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

	// All websockets requests are POSTs to a
	p.Handle("/web.Proxy/{method}", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(authHandler),
		negroni.Wrap(websocketsProxy(wsproxy)),
	)).Methods("POST")

	p.Handle("/frontend.js", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(authHandler),
		negroni.Wrap(http.HandlerFunc(homeHandler)),
	)).Methods("GET")

	p.Handle("/", negroni.New(
		setConf(conf),
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(authHandler),
		negroni.Wrap(http.HandlerFunc(homeHandler)),
	)).Methods("GET")

	addr := fmt.Sprintf("0.0.0.0:%s", conf.WebUIPort)
	httpsSrv := &http.Server{
		Addr:    addr,
		Handler: p,
		// Some security settings
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

	if conf.APIClientValidation {
		// Only start an external API listener if we're validating client keys.
		// Anything else is insecure. Note that the co-chair can still be
		// administered through the web ui, because that is protected by OAuth.
		go func() { pureGRPC <- grpcOnlyServer.Serve(lis) }()
	} else {
		logger.Info("we will not start an external grpc API listener because apiClientValidation is false")
	}

	go func() {
		logger.Info("Serving on https://" + addr)
		grpcAPI <- httpsSrv.ListenAndServeTLS(conf.WebUICert, conf.WebUIKey)
	}()

	l, err := backend.NewTCPForwarder(conf.ProxyCert, conf.ProxyKey, conf.ProxyPort)
	if err != nil {
		return err
	}
	// TCPForwarder routine
	go func() {
		for {
			tlsconn, err := l.Accept()
			if err != nil {
				logger.Errorf("proxy accept: %v", err)
				continue
			}
			// we accepted a tls connection, handle it asynchronously
			go func() {
				tlsconn.SetDeadline(time.Now().Add(3 * time.Second))
				buf := make([]byte, 4096)
				n, err := tlsconn.Read(buf)
				if err != nil {
					logger.Errorf("first read: %v", err)
					return
				}
				tlsconn.SetDeadline(time.Now().Add(5 * time.Second))

				var host string
				lines := strings.Split(string(buf[:n]), "\r\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "Host:") {
						host = strings.TrimSpace(strings.Split(line, ":")[1])
					}
				}

				// look up the domain in the db
				var bd backend.BackendData
				err = px.DB.One("Domain", host, &bd)
				if err != nil {
					if err == storm.ErrNotFound {
						logger.Debug("backend not found: ", host)
						return
					}
					logger.Error(err)
					return
				}
				logger.Debugf("dialing backend: %v", bd.IPs[0])
				bConn, err := tls.Dial("tcp", bd.IPs[0], &tls.Config{InsecureSkipVerify: true})
				if err != nil {
					logger.Errorf("dial backend: %v", err)
					return
				}
				bConn.SetDeadline(time.Now().Add(3 * time.Second))
				// our first write is the little buffer we read
				// from the incoming conn, just passing it along
				// after we've inspected it.
				_, err = bConn.Write(buf[:n])
				if err != nil {
					logger.Errorf("first write to backend: %v", err)
					return
				}

				var t = Tunnel{
					ErrorState:  nil,
					ErrorSig:    make(chan error),
					ServerConn:  tlsconn,
					BackendConn: bConn,
				}

				go t.pipe(tlsconn, bConn, "tlsconn->bConn")
				go t.pipe(bConn, tlsconn, "bConn->tslconn")
				logger.Debug("waiting")
				err = <-t.ErrorSig
				logger.Debugf("closing conns: %v", err)
				bConn.Close()
				tlsconn.Close()
			}()
		}
	}()

	if conf.ProxyInsecurePort != "" {
		// Unfortunately, we've decided to make a plaintext listener.
		go func() {
			fwdr, _ := backend.NewProxyForwarder(fmt.Sprintf("127.0.0.1:%s", conf.APIPort), logger)
			s := &http.Server{
				Addr:    fmt.Sprintf(":%s", conf.ProxyInsecurePort),
				Handler: fwdr,
			}
			proxy <- s.ListenAndServe()
		}()
	}
	// block
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

func (t *Tunnel) pipe(src, dst net.Conn, dir string) {

	buff := make([]byte, 0xffff)
	for {
		if t.ErrorState != nil {
			return
		}
		n, err := src.Read(buff)
		if err != nil {
			logger.Errorf("read failed %v", err)
			t.err(err)
			return
		}
		b := buff[:n]

		debg := true
		if debg {
			logger.Debugf("bufff %s %s", dir, string(b))
		}

		n, err = dst.Write(b)
		if err != nil {
			logger.Errorf("write failed %v", err)
			t.err(err)
			return
		}
	}
}

type Tunnel struct {
	ServerConn  net.Conn
	BackendConn net.Conn
	ErrorState  error
	ErrorSig    chan error
}

func (t *Tunnel) err(err error) {
	t.ErrorState = err
	t.ErrorSig <- err
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
