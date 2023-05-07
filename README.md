# teabag - [Static CMS](https://github.com/StaticJsCMS/static-cms)/[Decap](https://github.com/decaporg/decap-cms) OAuth provider for Gitea 

This is a lightweight Go server for handling OAuth flows with Gitea.

## Setup

### Manual deployment

Open the repo and build the service:

```
go build -o teabag .
```

Deploy the binary to your server. 

### Docker deployment

The official docker image is available under `ghcr.io/denyskon/teabag:latest`.

If you want to use docker compose, here is a suggested `docker-compose.yml`file.

```yaml
version: '2'
services:
  teabag:
    image: ghcr.io/denyskon/teabag
    restart: always
    environment:
      - TEABAG_PORT=3000
      - TEABAG_SESSION_SECRET=super-secret
      - TEABAG_GITEA_KEY=<KEY>
      - TEABAG_GITEA_SECRET=<SECRET>
      - TEABAG_GITEA_BASE_URL=https://gitea.company.com
      - TEABAG_GITEA_AUTH_URI=login/oauth/authorize
      - TEABAG_GITEA_TOKEN_URI=login/oauth/access_token
      - TEABAG_GITEA_USER_URI=api/v1/user
      - TEABAG_CALLBACK_URI=http://oauth.example.com:3000/callback
    ports:
      - "3000:3000"
```

## Config

The service needs some minimal configuration set before it can run. 
On the server or the location you are running the service, create a config file:

```bash
mkdir ./env
touch ./env/teabag.env
# OR
mkdir /etc/teabag
touch /etc/teabag/teabag.env
```

The config file is based on envfile. You can see a complete example in this repo at `./env/teabag.env.example`

```bash
HOST=localhost # The hostname to bind to
PORT=3000 # The port to serve on
SESSION_SECRET=super-secret # Used with OAuth provider sessions
```

There are some required settings to connect to Gitea:

```bash
# OAuth key and Ssecret generated on Gitea
GITEA_KEY=<KEY>
GITEA_SECRET=<SECRET>
# URL of Gitea instance
GITEA_BASE_URL=https://gitea.example.com
# endpoint URIs (see https://docs.gitea.com/development/oauth2-provider/)
GITEA_AUTH_URI=login/oauth/authorize
GITEA_TOKEN_URI=login/oauth/access_token
GITEA_USER_URI=api/v1/user
# callback URL, where users will be redirected after they authorise. Must contain the public URL of your teabag instance. This needs to match what was given when creating the OAuth application in Gitea.
CALLBACK_URI=http://localhost:3000/callback
```

You can also provide the config using environment variables. For that you need to prefix every variable with `TEABAG_`, e. g. `TEABAG_HOST=0.0.0.0`.

### Credits

Fork of https://github.com/donskifarrell/scm-oauth-provider
