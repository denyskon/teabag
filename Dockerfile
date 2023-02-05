LABEL org.opencontainers.image.source = "https://github.com/denyskon/teabag" 

FROM golang:1.19-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o ./teabag

EXPOSE 3000


CMD [ "./teabag" ]