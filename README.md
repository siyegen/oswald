Oswald, POM tracker
===================

WIP, don't expect it to work 100% yet

## How to build
Client: ```go build -o build/oswald ./client```

Server ```go build -o build/oswald-server ./server```

### How to run
```./oswald-server``` to start server

```./oswald start -name "Pom Name"``` to start a pom

```./oswald status``` to display time left in pom, or stats

```./oswald stop``` to cancel the currently running pom
