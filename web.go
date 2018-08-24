package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/anxiousmodernman/co-chair/config"
	"github.com/anxiousmodernman/co-chair/frontend/bundle"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

// Constants for getting/setting context.Context values.
const (
	ctxConfig ctxKey = iota
)

type ctxKey int

var indexTemplate = `
<p><a href="/auth/auth0">Log in with auth0</a></p>
`

// setConf captures the passed-in config and sets it on a request. We return
// the type our middleware framework expects. See negroni.New for details.
func setConf(conf config.Config) negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxConfig, conf)
		r = r.WithContext(ctx)
		next(w, r)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	conf := r.Context().Value(ctxConfig).(config.Config)
	domain := conf.Auth0Domain
	aud := ""

	oauthConf := &oauth2.Config{
		ClientID:     conf.Auth0ClientID,
		ClientSecret: conf.Auth0Secret,
		RedirectURL: fmt.Sprintf("https://%s:%s/auth/auth0/callback",
			conf.WebUIDomain, conf.WebUIPort),
		Scopes: []string{"openid", "profile"},
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
	url := oauthConf.AuthCodeURL(state, audience)
	logger.Debug("auth code url: ", url)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func loginLink(w http.ResponseWriter, r *http.Request) {
	t, _ := template.New("foo").Parse(indexTemplate)
	t.Execute(w, nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {

	conf := r.Context().Value(ctxConfig).(config.Config)
	domain := conf.Auth0Domain

	var u *url.URL
	u, err := url.Parse("https://" + domain)

	if err != nil {
		panic("boom")
	}
	session, err := Store.Get(r, "auth-session")
	if err != nil {
		w.WriteHeader(500)
		logger.Debug("tried to remove session but it's not there")
		return
	}

	// This is how you invalidate sessions. See docs for Save
	// http://www.gorillatoolkit.org/pkg/sessions
	session.Options.MaxAge = -1
	Store.Save(r, w, session)

	u.Path += "/v2/logout"

	parameters := url.Values{}
	parameters.Add("returnTo", fmt.Sprintf("https://%s:%s",
		conf.WebUIDomain, conf.WebUIPort))
	parameters.Add("client_id", conf.Auth0ClientID)
	u.RawQuery = parameters.Encode()

	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}

func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	conf := r.Context().Value(ctxConfig).(config.Config)
	domain := conf.Auth0Domain

	oauthConf := &oauth2.Config{
		ClientID:     conf.Auth0ClientID,
		ClientSecret: conf.Auth0Secret,
		RedirectURL: fmt.Sprintf("https://%s:%s/auth/auth0/callback",
			conf.WebUIDomain, conf.WebUIPort),
		Scopes: []string{"openid", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://" + domain + "/authorize",
			TokenURL: "https://" + domain + "/oauth/token",
		},
	}
	// Validate state before calling Exchange
	state := r.URL.Query().Get("state")
	// TODO do we remove state here, in this handler?
	// e.g. after it's done its work here
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
	token, err := oauthConf.Exchange(context.TODO(), code)
	if err != nil {
		logger.Error("oauth exchange failure: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Getting now the userInfo
	client := oauthConf.Client(context.TODO(), token)
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

func staticHandler(w http.ResponseWriter, r *http.Request) {
	http.FileServer(bundle.Assets).ServeHTTP(w, r)
}

func staticFromDiskHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		joined := filepath.Join(dir, r.URL.Path)
		logger.Info("path: ", joined)

		http.ServeFile(w, r, joined)
		//http.FileServer(http.Dir(dir))
	}
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
