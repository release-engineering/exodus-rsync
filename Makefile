default: exodus-rsync

exodus-rsync:
	go build ./cmd/exodus-rsync

check:
	go test -coverprofile=coverage.out ./...

htmlcov: check
	go tool cover -html=coverage.out

clean:
	rm -f exodus-rsync


.PHONY: check default clean exodus-rsync
