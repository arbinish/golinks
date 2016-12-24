# golinks
Yet another url shortener. A standalone, single node url shortener implemented 
in Go. Instead of a database, Go's native `gob` format is used. 

## Dependency
Only external dependency is `gorilla/mux`. To install use 
```bash
go get github.com/gorilla/mux
```

## Build

```bash
go get github.com/gorilla/mux
go build golinks.go

```


## Run

`./golinks`

Above will start a http server listening on port 8085.

## Usage

There are three apis, viz

   1. Set api - `curl http://0:8085/api/set/<hash> -d 'url=encoded_url'`
      - <hash> is any named hash. Instead of generating a hard to remember
      hash it is left for the user to come up with an easy to remember short
      hash
   2. Get api - `curl http://0:8085/api/get/<hash>`
      - returns record corresponding to the <hash> if one exists. Else it
        returns an empty record with empty `url` field, and created and update
        timestamps set to 0
   3. View api - `curl http://0:8085/v/<hash>`
       - redirects to saved url correspoding to `<hash>` else returns 404
         status with a NOT FOUND message
