// Copyright 2017 Johan Brandhorst. All Rights Reserved.
// See LICENSE for licensing terms.

package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	jwt "github.com/dgrijalva/jwt-go"

	"github.com/anxiousmodernman/goth"
	"github.com/anxiousmodernman/goth/gothic"
	"github.com/anxiousmodernman/goth/providers/auth0"
	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/johanbrandhorst/protobuf/wsproxy"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"gitlab.com/DSASanFrancisco/co-chair/backend"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/bundle"
	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
)

// TODO pass this down to my object
var logger *logrus.Logger

func init() {
	logger = logrus.StandardLogger()
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
		DisableSorting:  true,
	})
	// Should only be done from init functions
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(logger.Out, logger.Out, logger.Out))

	// GOTH INIT
	store := sessions.NewFilesystemStore(os.TempDir(), []byte("goth-example"))

	// set the maxLength of the cookies stored on the disk to a larger number to prevent issues with:
	// securecookie: the value is too long
	// when using OpenID Connect , since this can contain a large amount of extra information in the id_token

	// Note, when using the FilesystemStore only the session.ID is written to a browser cookie, so this is explicit for the storage on disk
	store.MaxLength(math.MaxInt64)

	gothic.Store = store
}

func main() {

	/* GOTH STUFF */
	goth.UseProviders(
		//Auth0 allocates domain per customer, a domain must be provided for auth0 to work
		auth0.New(
			os.Getenv("COCHAIR_AUTH0_CLIENTID"),
			os.Getenv("COCHAIR_AUTH0_SECRET"), "https://localhost:2016/auth/auth0/callback",
			os.Getenv("COCHAIR_AUTH0_DOMAIN")),
	)

	m := make(map[string]string)
	m["auth0"] = "Auth0"

	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	providerIndex := &ProviderIndex{Providers: ks, ProvidersMap: m}
	/* GOTH STUFF */

	// Proxy is our code that implements generated interface for server.
	prxy, err := backend.NewProxy("co-chair.db")
	if err != nil {
		log.Fatalf("proxy init: %v", err)
	}

	gs := grpc.NewServer()
	server.RegisterProxyServer(gs, prxy)
	wrappedServer := grpcweb.WrapServer(gs)

	clientCreds, err := credentials.NewClientTLSFromFile("./cert.pem", "")
	if err != nil {
		logger.WithError(err).Fatal("Failed to get local server client credentials, did you run `make generate_cert`?")
	}

	wsproxy := wsproxy.WrapServer(
		http.HandlerFunc(wrappedServer.ServeHTTP),
		wsproxy.WithLogger(logger),
		wsproxy.WithTransportCredentials(clientCreds))

	p := pat.New()
	p.Get("/auth/{provider}/callback", func(res http.ResponseWriter, req *http.Request) {

		user, err := gothic.CompleteUserAuth(res, req)
		if err != nil {
			fmt.Fprintln(res, err)
			return
		}
		t, _ := template.New("foo").Parse(userTemplate)
		t.Execute(res, user)
	})

	p.Get("/logout/{provider}", func(res http.ResponseWriter, req *http.Request) {
		gothic.Logout(res, req)
		res.Header().Set("Location", "/")
		res.WriteHeader(http.StatusTemporaryRedirect)
	})

	p.Get("/auth/{provider}", func(res http.ResponseWriter, req *http.Request) {
		// try to get the user without re-authenticating
		if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
			t, _ := template.New("foo").Parse(userTemplate)
			t.Execute(res, gothUser)
		} else {
			gothic.BeginAuthHandler(res, req)
		}
	})

	p.Get("/", func(res http.ResponseWriter, req *http.Request) {
		t, _ := template.New("foo").Parse(indexTemplate)
		t.Execute(res, providerIndex)
	})

	handler := func(resp http.ResponseWriter, req *http.Request) {
		// Redirect gRPC and gRPC-Web requests to the gRPC-Web Websocket Proxy server
		if req.ProtoMajor == 2 && strings.Contains(req.Header.Get("Content-Type"), "application/grpc") ||
			websocket.IsWebSocketUpgrade(req) {
			// auth?

			wsproxy.ServeHTTP(resp, req)
		} else {
			// Serve the GopherJS client
			http.FileServer(bundle.Assets).ServeHTTP(resp, req)
		}
	}

	// auth feature flag
	var withauth = false
	var h http.Handler
	if withauth {
		jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
			ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
				return []byte(os.Getenv("COCHAIR_AUTH0_SECRET")), nil
			},
			Extractor: extractor, // type TokenExtractor
			// TokenExtractor is a function that takes a request as input and returns
			// either a token or an error.  An error should only be returned if an attempt
			// to specify a token was found, but the information was somehow incorrectly
			// formed.  In the case where a token is simply not present, this should not
			// be treated as an error.  An empty string should be returned in that case.
			//type TokenExtractor func(r *http.Request) (string, error)

			// When set, the middleware verifies that tokens are signed with the
			// specific signing algorithm If the signing method is not constant
			// the ValidationKeyGetter callback can be used to implement additional checks
			// Important to avoid security issues described here:
			// https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
			SigningMethod: jwt.SigningMethodHS256,
		})

		h = jwtMiddleware.Handler(http.HandlerFunc(handler))
	} else {
		h = http.HandlerFunc(handler)
		// https://dsasf.auth0.com/login?client=xxx
	}
	_ = h

	addr := "localhost:2016"
	httpsSrv := &http.Server{
		Addr:    addr,
		Handler: h, //p,
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

	logger.Info("Serving on https://" + addr)
	logger.Fatal("handler exit: %v", httpsSrv.ListenAndServeTLS("./cert.pem", "./key.pem"))

}

var indexTemplate = `{{range $key,$value:=.Providers}}
<p><a href="/auth/{{$value}}">Log in with {{index $.ProvidersMap $value}}</a></p>
{{end}}`

var userTemplate = `
<p><a href="/logout/{{.Provider}}">logout</a></p>
<p>Name: {{.Name}} [{{.LastName}}, {{.FirstName}}]</p>
<p>Email: {{.Email}}</p>
<p>NickName: {{.NickName}}</p>
<p>Location: {{.Location}}</p>
<p>AvatarURL: {{.AvatarURL}} <img src="{{.AvatarURL}}"></p>
<p>Description: {{.Description}}</p>
<p>UserID: {{.UserID}}</p>
<p>AccessToken: {{.AccessToken}}</p>
<p>ExpiresAt: {{.ExpiresAt}}</p>
<p>RefreshToken: {{.RefreshToken}}</p>
`

type ProviderIndex struct {
	Providers    []string
	ProvidersMap map[string]string
}

func extractor(req *http.Request) (string, error) {
	if req == nil {
		return "", errors.New("no http request provided")
	}

	c, err := req.Cookie("auth0_gothic_session")
	if err != nil {
		return "", fmt.Errorf("cookie read: %v", err)
	}
	return c.Value, nil
}
