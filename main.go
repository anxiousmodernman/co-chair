package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/anxiousmodernman/goth/gothic"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/johanbrandhorst/protobuf/wsproxy"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/DSASanFrancisco/co-chair/backend"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/bundle"
	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
)

// Version is our software version.
var Version = "0.1.0"

var (
	// Store is our sessions store.
	Store *sessions.CookieStore
)

// TODO pass this down to my object
var logger *logrus.Logger

func init() {

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

	dbFlag := cli.StringFlag{
		Name:  "db",
		Usage: "path to db",
		Value: "co-chair.db",
	}

	apiCert := cli.StringFlag{
		Name:  "apiCert",
		Usage: "for mgmt api: path to pem encoded tls certificate",
		Value: "./cert.pem",
	}

	apiKey := cli.StringFlag{
		Name:  "apiKey",
		Usage: "for mgmt api: path to pem encoded tls private key",
		Value: "./key.pem",
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:  "serve",
			Flags: []cli.Flag{dbFlag, apiCert, apiKey},
			Action: func(ctx *cli.Context) error {
				return run(
					ctx.String("db"),
					ctx.String("apiCert"),
					ctx.String("apiKey"),
				)
			},
		},
	}

	app.Run(os.Args)
}

func run(dbPath, apiCert, apiKey string) error {

	// NewProxy gives us a Proxy, our concrete implementation of the
	// interface generated by the grpc protobuf compiler.
	px, err := backend.NewProxy(dbPath)
	if err != nil {
		log.Fatalf("proxy init: %v", err)
	}

	// Pure-gRPC management API
	grpcOnlyServer := grpc.NewServer()
	// passing px two places... ruh roh?
	server.RegisterProxyServer(grpcOnlyServer, px)
	lis, err := net.Listen("tcp", "127.0.0.1:1917")
	if err != nil {
		return fmt.Errorf("listener error: %v", err)
	}
	go func() { grpcOnlyServer.Serve(lis) }()

	// gRPC over websockets management API
	gs := grpc.NewServer()
	server.RegisterProxyServer(gs, px)
	wrappedServer := grpcweb.WrapServer(gs)

	clientCreds, err := credentials.NewClientTLSFromFile(apiCert, "")
	if err != nil {
		logger.WithError(err).Fatal("Failed to get local server client credentials, did you run `make generate_cert`?")
	}

	wsproxy := wsproxy.WrapServer(
		http.HandlerFunc(wrappedServer.ServeHTTP),
		wsproxy.WithLogger(logger),
		wsproxy.WithTransportCredentials(clientCreds))

	// Note: routes are evaluated in the order they're defined.
	p := mux.NewRouter()

	p.Handle("/login", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(loginLink)),
	)).Methods("GET")

	p.Handle("/auth/{provider}/callback", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(oauthCallbackHandler)),
	)).Methods("GET")

	p.Handle("/auth/{provider}", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(loginHandler)),
	)).Methods("GET")

	p.Handle("/logout/{provider}", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.Wrap(http.HandlerFunc(logoutHandler)),
	)).Methods("GET")

	// All websockets requests
	p.Handle("/web.Proxy/{method}", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(IsAuthenticated),
		negroni.Wrap(websocketsProxy(wsproxy)),
	)).Methods("POST")

	p.Handle("/", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(IsAuthenticated),
		negroni.Wrap(http.HandlerFunc(homeHandler)),
	)).Methods("GET")

	p.Handle("/frontend.js", negroni.New(
		negroni.HandlerFunc(withLog),
		negroni.HandlerFunc(IsAuthenticated),
		negroni.Wrap(http.HandlerFunc(homeHandler)),
	)).Methods("GET")

	addr := "localhost:2016"
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

	go func() {
		logger.Info("Serving on https://" + addr)
		grpcAPI <- httpsSrv.ListenAndServeTLS("./cert.pem", "./key.pem")
	}()

	fwdr, _ := backend.NewProxyForwarder("127.0.0.1:1917", logger)

	go func() {

		s := &http.Server{
			Addr:    ":8080",
			Handler: fwdr,
		}
		proxy <- s.ListenAndServe()
	}()

	// block until we get an error on a channel
	for {
		select {
		case err := <-grpcAPI:
			return err
		case err := <-proxy:
			return err
		}
	}

}

var indexTemplate = `
<p><a href="/auth/auth0">Log in with auth0</a></p>
`

func loginHandler(w http.ResponseWriter, r *http.Request) {
	domain := "dsasf.auth0.com"
	aud := ""

	conf := &oauth2.Config{
		ClientID:     os.Getenv("COCHAIR_AUTH0_CLIENTID"),
		ClientSecret: os.Getenv("COCHAIR_AUTH0_SECRET"),
		RedirectURL:  "https://localhost:2016/auth/auth0/callback",
		Scopes:       []string{"openid", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://" + domain + "/authorize",
			TokenURL: "https://" + domain + "/oauth/token",
		},
	}

	if aud == "" {
		aud = "https://" + domain + "/userinfo"
	}

	// Generate random state
	b := make([]byte, 32)
	rand.Read(b)
	state := base64.StdEncoding.EncodeToString(b)

	session, err := Store.Get(r, "state")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["state"] = state
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	audience := oauth2.SetAuthURLParam("audience", aud)
	// add "code" here?
	url := conf.AuthCodeURL(state, audience)
	logger.Debug("auth code url: ", url)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func loginLink(w http.ResponseWriter, r *http.Request) {
	t, _ := template.New("foo").Parse(indexTemplate)
	t.Execute(w, nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {

	domain := os.Getenv("COCHAIR_AUTH0_DOMAIN")

	var u *url.URL
	u, err := url.Parse("https://" + domain)

	if err != nil {
		panic("boom")
	}

	u.Path += "/auth/auth0/logout"
	parameters := url.Values{}
	parameters.Add("returnTo", "https://localhost:2016")
	parameters.Add("client_id", os.Getenv("COCHAIR_AUTH0_CLIENTID"))
	u.RawQuery = parameters.Encode()

	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}

func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	domain := "dsasf.auth0.com"

	conf := &oauth2.Config{
		ClientID:     os.Getenv("COCHAIR_AUTH0_CLIENTID"),
		ClientSecret: os.Getenv("COCHAIR_AUTH0_SECRET"),
		RedirectURL:  "https://localhost:2016/auth/auth0/callback",
		Scopes:       []string{"openid", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://" + domain + "/authorize",
			TokenURL: "https://" + domain + "/oauth/token",
		},
	}
	// Validate state before calling Exchange
	state := r.URL.Query().Get("state")
	session, err := Store.Get(r, "state")
	if err != nil {
		logger.Errorf("could not get session state: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if state != session.Values["state"] {
		http.Error(w, "Invalid state parameter", http.StatusInternalServerError)
		return
	}

	var code string
	code = r.URL.Query().Get("code")
	if code == "" {
		logger.Error("code is not in query params")
	}
	code = r.FormValue("code")
	if code == "" {
		logger.Error("code is not in form")
	}
	idToken := r.URL.Query().Get("id_token")
	if idToken != "" {
		logger.Debugf("got a token: %s", idToken)
	}

	// package oauth2 docs:
	// The code will be in the *http.Request.FormValue("code").
	token, err := conf.Exchange(context.TODO(), code)
	if err != nil {
		logger.Error("oauth exchange failure: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Getting now the userInfo
	client := conf.Client(context.TODO(), token)
	resp, err := client.Get("https://" + domain + "/userinfo")
	if err != nil {
		logger.Errorf("error calling userinfo endpoint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	var profile map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		logger.Errorf("could not decode userinfo response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, err = Store.Get(r, "auth-session")
	if err != nil {
		logger.Errorf("could not get session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Values["id_token"] = token.Extra("id_token")
	session.Values["access_token"] = token.AccessToken
	session.Values["profile"] = profile
	err = session.Save(r, w)
	if err != nil {
		logger.Errorf("could not save session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect to logged in page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.FileServer(bundle.Assets).ServeHTTP(w, r)
}

func websocketsProxy(wsproxy http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") ||
			websocket.IsWebSocketUpgrade(r) {
			wsproxy.ServeHTTP(w, r)
		}
	}
}

func IsAuthenticated(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	session, err := Store.Get(r, "auth-session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, ok := session.Values["profile"]; !ok {
		logger.Errorf("session profile not found; redirecting.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	} else {
		logger.Info("auth passed")
		next(w, r)
	}
}

func withLog(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	logger.Debug("path:", r.URL.Path)
	next(w, r)
}
