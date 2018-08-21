test:
	go test -v ./...

run:
	go run main.go

stress:
	@go run main.go &
	@echo "waiting a few seconds to allow app to start up" && sleep 5
	@vegeta attack -rate=50/1s -duration=30s -targets=./load-test/targets.txt | vegeta report
	@pkill go main # kill two separate processes: 'go' (go run main.go) and 'main' (/var/folders/.../go-build.../.../exe/main)
