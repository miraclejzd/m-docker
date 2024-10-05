.PHONY: demo
demo: build
	@echo "\033[34m🚀 running demo with command: '\033[33msudo m-docker run -it /bin/bash\033[34m' 🚀\033[0m"
	@sudo m-docker run -it /bin/bash

.PHONY: build
build: required
	@go build -o m-docker main.go
	@sudo cp m-docker /usr/local/bin
	@echo "\033[32m🎉 build m-docker binary successfully \033[35m🎉\033[0m\n"

.PHONY: required
required: go-mod
	@echo "\033[32m✨ \033[36mAll requirements met \033[32m✨\033[0m\n"

.PHONY: go-mod
go-mod:
	@go mod tidy
	@go mod vendor
	@echo "\033[32m - \033[36m✅ Go modules tidied and vendored \033[32m\033[0m"