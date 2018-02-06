GO=govendor
BUILD=$(GO) build
CLEAN=$(GO) clean

OUT = rp

all: build test

build: check-deps
	$(BUILD) -o $(OUT) .

run: check-deps
	$(BUILD) -o $(OUT)
	./$(OUT)

test: check-deps
	@docker-compose up --build -d
	docker exec -it rp govendor test github.com/jeloou/rp/proxy -- -v
	govendor test github.com/jeloou/rp/proxy -v 2>/dev/null
	@docker-compose rm -s -f

clean:
	$(CLEAN)
	rm -f $(OUT)

check-deps:
	@type $(GO) >/dev/null 2>&1 || go get -u github.com/kardianos/govendor
