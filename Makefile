all: bin/linux_amd64 bin/win_amd64.exe bin/mac_arm64 bin/mac_amd64

clean:
	@rm bin/*

bin/linux_amd64: main.go
	GOOS=linux GOARCH=amd64 go build -o bin/linux_amd64 main.go
bin/win_amd64.exe: main.go
	GOOS=windows GOARCH=amd64 go build -o bin/win_amd64.exe main.go
bin/mac_arm64: main.go
	GOOS=darwin GOARCH=arm64 go build -o bin/mac_arm64 main.go
bin/mac_amd64: main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/mac_amd64 main.go
