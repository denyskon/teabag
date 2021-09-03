package main

import (
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
	serviceName = "netlify-cms-oauth-provider"
	msgTemplate = `<!DOCTYPE html><html><head></head><body>{{.}}</body></html>`

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
		key, secret, baseURL, callbackURL, authURI, accessTokenURI, userURI string
	}

	log.Info("initialising providers")

	getProviderSetings := func(name string) settings {
		baseURL := config.GetString(name + ".baseURL")
		return settings{
			key:            config.GetString(name + ".key"),
			secret:         config.GetString(name + ".secret"),
			baseURL:        baseURL,
			authURI:        fmt.Sprintf("%s/%s", baseURL, config.GetString(name+".authURI")),
			accessTokenURI: fmt.Sprintf("%s/%s", baseURL, config.GetString(name+".accessTokenURI")),
			userURI:        fmt.Sprintf("%s/%s", baseURL, config.GetString(name+".userURI")),
			callbackURL:    config.GetString(name + ".callbackURI"),
		}
	}

	if config.InConfig("gitea") {
		log.Info("- adding gitea provider")
		var p goth.Provider
		s := getProviderSetings("gitea")
		if s.authURI != "" {
			log.Infof("-- with custom settings %+v", s)
			p = gitea.NewCustomisedURL(s.key, s.secret, s.callbackURL, s.authURI, s.accessTokenURI, s.userURI)
		} else {
			p = gitea.New(s.key, s.secret, s.callbackURL)
		}
		providers = append(providers, p)
	}

	if config.InConfig("gitlab") {
		log.Info("- adding gitlab provider")
		var p goth.Provider
		s := getProviderSetings("gitlab")
		if s.authURI != "" {
			log.Infof("-- with custom settings %+v", s)
			p = gitlab.NewCustomisedURL(s.key, s.secret, s.callbackURL, s.authURI, s.accessTokenURI, s.userURI)
		} else {
			p = gitlab.New(s.key, s.secret, s.callbackURL)
		}
		providers = append(providers, p)
	}

	if config.InConfig("github") {
		log.Info("- adding github provider")
		var p goth.Provider
		s := getProviderSetings("github")
		p = github.New(s.key, s.secret, s.callbackURL)
		providers = append(providers, p)
	}

	if config.InConfig("bitbucket") {
		log.Info("- adding bitbucket provider")
		var p goth.Provider
		s := getProviderSetings("bitbucket")
		p = bitbucket.New(s.key, s.secret, s.callbackURL)
		providers = append(providers, p)
	}

	gothic.Store = sessions.NewCookieStore([]byte(config.GetString("server.sessionSecret")))
	goth.UseProviders(providers...)
}

const (
	script = `<!DOCTYPE html><html><head><script>
  if (!window.opener) {
    window.opener = {
      postMessage: function(action, origin) {
        console.log(action, origin);
      }
    }
  }
  (function(status, provider, result) {
    function recieveMessage(e) {
      console.log("Recieve message:", e);
      // send message to main window with da app
	  console.log("Sending message:", "authorization:" + provider + ":" + status + ":" + result, e.origin)
      window.opener.postMessage(
        "authorization:" + provider + ":" + status + ":" + result,
        e.origin
      );
    }
    window.addEventListener("message", recieveMessage, false);

    // Start handshare with parent
    console.log("Sending message:", "authorizing:" + provider, "*")
    window.opener.postMessage(
      "authorizing:" + provider,
      "*"
    );
  })(%#v, %#v, %#v)
  </script></head><body></body></html>`
)

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
		log.Info("logged in user")
		// t, _ := template.New("msg").Parse(msgTemplate)
		// t.Execute(w, fmt.Sprintf("Connected with UserID '%s'", user.UserID))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		result := fmt.Sprintf(`{"token":"%s", "provider":"%s"}`, user.AccessToken, user.Provider)

		log.Info("details: %+v", user)
		w.Write([]byte(fmt.Sprintf(script, "success", provider, result)))
	}).Methods("GET")

	// redirect to correct auth/{provider} URL if Auth request is submited with a query param '&provider=X'
	// TODO: Remove hardcoded http://
	r.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		host := net.JoinHostPort(config.GetString("server.host"), config.GetString("server.port"))
		URL := fmt.Sprintf("http://%s/auth/%s", host, r.FormValue("provider"))

		log.Infof("redirecting to '%s'\n", URL)
		http.Redirect(w, r, URL, http.StatusTemporaryRedirect)
	}).Methods("GET")

	r.HandleFunc("/auth/{provider}", func(w http.ResponseWriter, r *http.Request) {
		log.Infof("handling auth provider request '%s'\n", r)
		if gothUser, err := gothic.CompleteUserAuth(w, r); err == nil {
			t, _ := template.New("msg").Parse(msgTemplate)
			t.Execute(w, fmt.Sprintf("Connected to existing session with UserID '%s'", gothUser.UserID))
		} else {
			gothic.BeginAuthHandler(w, r)
		}
	}).Methods("GET")

	r.HandleFunc("/logout/{provider}", func(w http.ResponseWriter, r *http.Request) {
		log.Infof("logout with '%s'\n", r)
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
