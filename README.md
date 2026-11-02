# pitaya

<p align="center">
  <a href="https://github.com/cuihairu/pitaya/releases"><img src="https://img.shields.io/badge/status-in%20development-orange" alt="Status"></a> <a href="https://cuihairu.github.io/pitaya/"><img src="https://img.shields.io/badge/docs-blue" alt="docs"></a>
  <a href="https://github.com/cuihairu/pitaya/actions/workflows/tests.yaml"><img src="https://github.com/cuihairu/pitaya/actions/workflows/tests.yaml/badge.svg" alt="Build Status"></a>
  <a href="https://coveralls.io/github/cuihairu/pitaya?branch=main"><img src="https://coveralls.io/repos/github/cuihairu/pitaya/badge.svg?branch=main" alt="Coverage Status"></a>
  <a href="https://pkg.go.dev/github.com/topfreegames/pitaya/v3"><img src="https://pkg.go.dev/badge/github.com/topfreegames/pitaya/v3.svg" alt="GoDoc"></a>
  <a href="https://pitaya.readthedocs.io/en/latest/"><img src="https://readthedocs.org/projects/pitaya/badge/?version=latest" alt="Docs"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT licensed"></a>
</p>

Pitaya is a scalable, fast and lightweight game server framework with clustering support and client libraries for iOS, Android, Unity and others through the [C SDK](https://github.com/topfreegames/libpitaya).
It provides a basic development framework for distributed multiplayer games and server-side applications.

## Getting Started

### Prerequisites

* [Go](https://golang.org/) >= 1.25.4
* [etcd](https://github.com/etcd-io/etcd) (optional, used for service discovery)
* [nats](https://github.com/nats-io/nats.go) (optional, used for sending and receiving rpc)
* [docker](https://www.docker.com) (optional, used for running etcd and nats dependencies on containers)

### Installing
clone the repo
```
git clone https://github.com/cuihairu/pitaya.git
```
setup pitaya dependencies
```
make setup
```

### Hacking pitaya

Here's one example of running Pitaya:

Start etcd (This command requires docker-compose and will run an etcd container locally. An etcd may be run without docker if preferred.)
```
cd ./examples/testing && docker compose up -d etcd
```
run the connector frontend server from cluster_grpc example
```
make run-cluster-grpc-example-connector
```
run the room backend server from the cluster_grpc example
```
make run-cluster-grpc-example-room
```

Now there should be 2 pitaya servers running, a frontend connector and a backend room. To send requests, use the REPL client bundled in this repo, [pitaya-cli](./cmd).

```
$ pitaya repl
Pitaya REPL Client
>>> connect localhost:3250
connected!
>>> request room.room.entry
>>> sv-> {"code":0,"result":"ok"}
```

## Running the tests
```
make test
```
This command will run both unit and e2e tests.

## Contributing
#TODO

## Authors
* **TFG Co** - Initial work

## License
[MIT License](./LICENSE)

## Acknowledgements
* [nano](https://github.com/lonnng/nano) authors for building the framework pitaya is based on.
* [pomelo](https://github.com/NetEase/pomelo) authors for the inspiration on the distributed design and protocol

## Security
If you have found a security vulnerability, please email security@tfgco.com

## Resources
- Other pitaya-related projects
  + [libpitaya-cluster](https://github.com/topfreegames/libpitaya-cluster)
  + [libpitaya](https://github.com/topfreegames/libpitaya)
  + [pitaya-admin](https://github.com/topfreegames/pitaya-admin)
  + [pitaya-bot](https://github.com/topfreegames/pitaya-bot)
  + [pitaya-protos](https://github.com/topfreegames/pitaya-protos)

- Documents
  + [API Reference](https://pkg.go.dev/github.com/topfreegames/pitaya/v3)
  + [REPL CLI](./docs/cli.md)
  + [In-depth documentation](https://pitaya.readthedocs.io/en/latest/)

- Demo
  + [Implement a chat room in ~100 lines with pitaya and WebSocket](./examples/demo/chat) (adapted from [nano](https://github.com/lonnng/nano)'s example)
