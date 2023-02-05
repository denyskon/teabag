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
	"github.com/markbates/goth/providers/gitea"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	serviceName = "teabag"
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
	config.SetConfigType("env")
	config.SetConfigName("teabag")
	config.AddConfigPath("./env/")
	config.AddConfigPath("/etc/teabag/")
	config.SetEnvPrefix("teabag")
	config.AutomaticEnv()

	if err := config.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Infoln("config file not found, falling back to evironment variables")
		} else {
			log.Fatalf("error loading configuration: %v", err)
		}
	}

}

func initProvider() {
	var (
		providers []goth.Provider
	)

	type settings struct {
		key, secret, BaseURL, CallbackURL, AuthURI, AccessTokenURI, UserURI string
	}

	log.Info("initialising provider")

	getProviderSetings := func(name string) settings {
		baseURL := config.GetString(name + "_base_url")
		return settings{
			key:            config.GetString(name + "_key"),
			secret:         config.GetString(name + "_secret"),
			BaseURL:        baseURL,
			AuthURI:        fmt.Sprintf("%s/%s", baseURL, config.GetString(name+"_auth_uri")),
			AccessTokenURI: fmt.Sprintf("%s/%s", baseURL, config.GetString(name+"_token_uri")),
			UserURI:        fmt.Sprintf("%s/%s", baseURL, config.GetString(name+"_user_uri")),
			CallbackURL:    config.GetString("callback_uri"),
		}
	}

	log.Info("- adding gitea connector")
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

	gothic.Store = sessions.NewCookieStore([]byte(config.GetString("session_secret")))
	goth.UseProviders(providers...)
}

func main() {
	log = logrus.New().WithFields(logrus.Fields{
		"service": serviceName,
	})
	log.Info("starting up service")

	initConfig()
	initProvider()

	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, _ := template.New("msg").Parse(msgTemplate)
		t.Execute(w, fmt.Sprintf("Connected to %s", serviceName))
	})

	r.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			log.Errorf("callback: CompleteUserAuth failed %v", err)
			return
		}

		log.Infoln("logged in user to gitea")
		t, _ := template.New("res").Parse(resTemplate)

		data := struct {
			Provider    string
			OAuthResult string
		}{
			Provider:    fmt.Sprintf(`authorizing:gitea`),
			OAuthResult: fmt.Sprintf(`authorization:gitea:%s:{"token":"%s", "provider":"%s"}`, "success", user.AccessToken, user.Provider),
		}
		t.Execute(w, data)
	}).Methods("GET")

	r.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		log.Infoln("handling auth provider request for gitea")

		if gothUser, err := gothic.CompleteUserAuth(w, r); err == nil {
			t, _ := template.New("msg").Parse(msgTemplate)
			t.Execute(w, fmt.Sprintf("Connected to existing session with UserID '%s'", gothUser.UserID))
		} else {
			gothic.BeginAuthHandler(w, r)
		}
	}).Methods("GET")

	r.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		log.Infoln("logout from gitea")

		gothic.Logout(w, r)
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}).Methods("GET")

	http.Handle("/", r)

	log.Infof("listening on %s:%d",
		config.GetString("host"),
		config.GetInt("port"),
	)

	srv := &http.Server{
		Handler:      r,
		Addr:         net.JoinHostPort(config.GetString("host"), config.GetString("port")),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
