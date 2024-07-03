mkdir -p bin
GOOS=windows GOARCH=amd64 go build -o bin/server-go.exe server.go
GOOS=windows GOARCH=amd64 go build -o bin/client-go.exe server.go
GOOS=windows GOARCH=amd64 go build -o bin/client-simple-go.exe client-simple.go
GOOS=linux GOARCH=amd64 go build -o bin/server-go server.go
GOOS=linux GOARCH=amd64 go build -o bin/client-go client.go
GOOS=linux GOARCH=amd64 go build -o bin/client-simple-go client-simple.go
