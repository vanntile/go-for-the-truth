# Go for The Truth

Small Go server for a real-or-fake quiz app using echo, templ, tailwind, and sqlite.

## How to run in dev


```sh
cp .env.example .env
# configure .env

# install air
go install github.com/air-verse/air@latest

# install templ
go install github.com/a-h/templ/cmd/templ@latest

# install tailwind binary from https://github.com/tailwindlabs/tailwindcss/releases/latest

# build and run with air watcher
air
```

# How to run in prod

```sh
# build binary (only for x86 linux)
CGO_ENABLED=1 go build -tags "linux" -ldflags "-s -w" main.go

# get lego binary
# https://go-acme.github.io/lego/installation/index.html

# get TLS certificates for domain
lego --email="you@example.com" --domains="example.com" --http run

# copy .env, time.so, .lego, main and web/public to the server

# set ownership and restricted permissions

# remove the GO_ENV variable from .env

# set path to server certificate, key and set address to `:443`

# don't run binary with sudo, just give access to lower ports
sudo setcap CAP_NET_BIND_SERVICE=+eip /path/to/main

# run with environment on bash using nohup
env $(cat .env | xargs) nohup ./main &

# upload some questions on the admin page

# ready to use
```

