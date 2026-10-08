BINARY := forge-monitor
PKG := ./cmd/forge-monitor
LDFLAGS := -s -w

.PHONY: build test tidy fmt vet dist clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 $(PKG)
	cd dist && shasum -a 256 $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 > checksums.txt

clean:
	rm -rf dist $(BINARY)
