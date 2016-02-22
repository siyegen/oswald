Oswald, POM tracker
===================

WIP, don't expect it to work 100% yet

## Install
#### For dev
- clone project into your workspace
- Install bolt: ```go get github.com/boltdb/bolt/...```
- make the directory for bolt: ```mkdir dev_db```
- Follow build instructions below

## How to build
Client: ```go build -o build/oswald ./client```

Server ```go build -o build/oswald-server ./server```

### How to run
```./oswald-server``` to start server

### TODO

```./oswald start -name "pom_name"``` to start a pom

```./oswald status``` to display time left in pom, or stats

```./oswald stop``` to cancel the currently running pom
