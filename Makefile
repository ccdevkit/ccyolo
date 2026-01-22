BINARY_NAME=ccyolo
BUILD_DIR=bin
INSTALL_PATH=/usr/local/bin
IMAGE_NAME=ccyolo:latest

.PHONY: build install uninstall clean docker run

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ccyolo

docker:
	docker build -t $(IMAGE_NAME) .

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

uninstall:
	rm -f $(INSTALL_PATH)/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)
