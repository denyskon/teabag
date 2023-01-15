package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/bitbucket"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/gitlab"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	serviceName = "oauth-provider"
	msgTemplate = `<!DOCTYPE html><html><head></head><body>{{.}}</body></html>`
	resTemplate = `
		<!DOCTYPE html><html><head></head><body>
			<script>
				function recieveMsg(e) {
					window.opener.postMessage("{{.OAuthResult}}", e.origin);
				}
				window.addEventListener("message", recieveMsg, false);
				window.opener.postMessage("{{.Provider}}", "*");
			</script>
		</body></html>`

	log    *logrus.Entry
	config *viper.Viper
)

func initConfig() {
	config = viper.New()
	config.SetConfigType("toml")
	config.SetConfigFile("./env/config")
	config.AutomaticEnv()

	if err := config.ReadInConfig(); err != nil {
		log.Fatalf("error loading configuration: %v", err)
	}
}

func initProviders() {
	var (
		providers []goth.Provider
	)

	type settings struct {
		key, secret, BaseURL, CallbackURL, AuthURI, AccessTokenURI, UserURI string
	}

	log.Info("initialising providers")

	getProviderSetings := func(name string) settings {
		baseURL := config.GetString(name + ".baseURL")
		return settings{
			key:            config.GetString(name + ".key"),
			secret:         config.GetString(name + ".secret"),
			BaseURL:        baseURL,
			AuthURI:        fmt.Sprintf("%s/%s", baseURL, config.GetString(name+".authURI")),
			AccessTokenURI: fmt.Sprintf("%s/%s", baseURL, config.GetString(name+".accessTokenURI")),
			UserURI:        fmt.Sprintf("%s/%s", baseURL, config.GetString(name+".userURI")),
			CallbackURL:    config.GetString(name + ".callbackURI"),
		}
	}

	if config.InConfig("gitea") {
		log.Info("- adding gitea provider")
		var p goth.Provider
		s := getProviderSetings("gitea")
		if s.AuthURI != "" {
			out, _ := json.MarshalIndent(s, "", "  ")
			log.Infof("-- with custom settings %s", string(out))
			p = gitea.NewCustomisedURL(s.key, s.secret, s.CallbackURL, s.AuthURI, s.AccessTokenURI, s.UserURI)
		} else {
			p = gitea.New(s.key, s.secret, s.CallbackURL)
		}
		providers = append(providers, p)
	}

	if config.InConfig("gitlab") {
		log.Info("- adding gitlab provider")
		var p goth.Provider
		s := getProviderSetings("gitlab")
		if s.AuthURI != "" {
			out, _ := json.MarshalIndent(s, "", "  ")
			log.Infof("-- with custom settings %s", string(out))
			p = gitlab.NewCustomisedURL(s.key, s.secret, s.CallbackURL, s.AuthURI, s.AccessTokenURI, s.UserURI)
		} else {
			p = gitlab.New(s.key, s.secret, s.CallbackURL)
		}
		providers = append(providers, p)
	}

	if config.InConfig("github") {
		log.Info("- adding github provider")
		var p goth.Provider
		s := getProviderSetings("github")
		p = github.New(s.key, s.secret, s.CallbackURL)
		providers = append(providers, p)
	}

	if config.InConfig("bitbucket") {
		log.Info("- adding bitbucket provider")
		var p goth.Provider
		s := getProviderSetings("bitbucket")
		p = bitbucket.New(s.key, s.secret, s.CallbackURL)
		providers = append(providers, p)
	}

	gothic.Store = sessions.NewCookieStore([]byte(config.GetString("server.sessionSecret")))
	goth.UseProviders(providers...)
}

func main() {
	log = logrus.New().WithFields(logrus.Fields{
		"service": serviceName,
	})
	log.Info("starting up service")

	initConfig()
	initProviders()

	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, _ := template.New("msg").Parse(msgTemplate)
		t.Execute(w, fmt.Sprintf("Connected to %s", serviceName))
	})

	r.HandleFunc("/callback/{provider}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		provider, err := gothic.GetProviderName(r)
		if err != nil {
			log.Errorf("callback: GetProviderName failed %v", err)
			return
		}

		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			log.Errorf("callback: CompleteUserAuth failed %v", err)
			return
		}

		log.Infof("logged in user to '%s'\n", vars["provider"])
		t, _ := template.New("res").Parse(resTemplate)

		data := struct {
			Provider    string
			OAuthResult string
		}{
			Provider:    fmt.Sprintf(`authorizing:%s`, provider),
			OAuthResult: fmt.Sprintf(`authorization:%s:%s:{"token":"%s", "provider":"%s"}`, provider, "success", user.AccessToken, user.Provider),
		}
		t.Execute(w, data)
	}).Methods("GET")

	// redirect to correct auth/{provider} URL if Auth request is submited with a query param '&provider=X'
	// TODO: Remove hardcoded http://
	r.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		proto := config.GetString("server.publicProto")
		host := net.JoinHostPort(config.GetString("server.host"), config.GetString("server.port"))
		URL := fmt.Sprintf("%s://%s/auth/%s", proto, host, r.FormValue("provider"))

		log.Infof("redirecting to '%s'\n", URL)
		http.Redirect(w, r, URL, http.StatusTemporaryRedirect)
	}).Methods("GET")

	r.HandleFunc("/auth/{provider}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Infof("handling auth provider request '%s'\n", vars["provider"])

		if gothUser, err := gothic.CompleteUserAuth(w, r); err == nil {
			t, _ := template.New("msg").Parse(msgTemplate)
			t.Execute(w, fmt.Sprintf("Connected to existing session with UserID '%s'", gothUser.UserID))
		} else {
			gothic.BeginAuthHandler(w, r)
		}
	}).Methods("GET")

	r.HandleFunc("/logout/{provider}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Infof("logout from '%s'\n", vars["provider"])

		gothic.Logout(w, r)
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}).Methods("GET")

	http.Handle("/", r)

	log.Infof("listening on %s:%d",
		config.GetString("server.host"),
		config.GetInt("server.port"),
	)

	srv := &http.Server{
		Handler:      r,
		Addr:         net.JoinHostPort(config.GetString("server.host"), config.GetString("server.port")),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
