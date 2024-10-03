.PHONY: demo
demo: build
	@echo "\033[34m🚀 running demo with command: 'sudo m-docker run -it /bin/bash' 🚀\033[0m"
	@sudo m-docker run -it /bin/bash

.PHONY: build
build: required
	@go build -o m-docker main.go
	@sudo cp m-docker /usr/local/bin
	@echo "\033[32m✨ build m-docker binary successfully ✨\033[0m\n"

.PHONY: required
required: main.go
	@echo "Dependencies attached:"
	@for dep in $^; do \
		if [ -d "$$dep" ]; then \
			echo " - $$dep/"; \
		else \
			echo " - $$dep"; \
		fi; \
	done

