# SCM OAuth Provider 

This is a lightweight Go server for handling OAuth flows with Gitea, Gitlab, Bitbucket, GitHub.

> Note: My primary use case is providing OAuth between Gitea and NetlifyCMS. Other SCMs are untested as of now.

## Setup

Open the repo and build the service:

```
go build -o oauth-provider .
```

Deploy the binary to your server. 

> Dockerfile is coming soon


## Config

The service needs some minimal configuration set before it can run. 
On the server or the location you are running the service, create a config file:

```
mkdir ./env
touch ./env/config
```

The config file is TOML based. You can see a complete example in this repo at `./env/sample.config`

```
[runtime]
# Not used anywhere yet, for information only
environment="development" 

[server]
# The hostname to serve from; Your external app will connect to the OAuth provider via this URL
host="localhost" 
# The port to serve from; Used in conjunction with [server.host] to create a complete URL
port="3000"
# Used with OAuth provider sessions
sessionSecret="super-secret"
```

For each CMS, there are some required settings:

```
[gitea]
# OAuth Key and Secret generated on the SCM site
key="<KEY>"
secret="<SECRET>"
# URL of the SCM instance
baseUrl="https://gitea.company.com"
# URI of the authorize endpoint (e.g for Gitea, this is shown when creating the OAuth application)
authUri="login/oauth/authorize"
# URI of the access_token endpoint (e.g for Gitea, this is shown when creating the OAuth application)
accessTokenUri="login/oauth/access_token"
# URI of the authorize endpoint if overridden (e.g for Gitea, this is shown when creating the OAuth application)
userUri="api/v1/user"
# Callback URL for the SCM, where it will redirect the user after they authorise. This needs to match what was given when creating the OAuth application.
callbackUri="http://localhost:3000/callback/gitea"
```


### Credits

Inspiration taken from https://github.com/igk1972/netlify-cms-oauth-provider-go