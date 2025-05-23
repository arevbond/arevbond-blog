run:
	docker compose -f compose.dev.yml up
restart:
	docker compose -f compose.dev.yml restart app
cover:
	go test -coverprofile=test-cover/coverage.out ./... &&	go tool cover -html=test-cover/coverage.out -o test-cover/coverage.html
