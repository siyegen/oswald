Oswald, POM tracker
===================

WIP, don't expect it to work 100% yet

## Install
- Install bolt: ```go get github.com/boltdb/bolt/...```
- Install oswald: ```go get github.com/siyegen/oswald/...```

## How To Run

### Server
```./oswald-server``` to start server

### Client
```./oswald-client -start "pom_name"``` to start a pom

```./oswald-client -status``` to display time left in pom, or stats

```./oswald-client -stop``` to cancel the currently running pom

```./oswald-client -help``` to see all commands

## Contributing

### How to build
Client: ```go build -o build/oswald-client ./oswald-client```

Server ```go build -o build/oswald-server ./oswald-server```
