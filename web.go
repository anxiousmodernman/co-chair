package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/bundle"
	"golang.org/x/oauth2"
)

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
