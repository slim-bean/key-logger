BINARY       := key-logger
INSTALL_DIR  := /usr/local/bin
PLIST_LABEL  := com.keylogger.agent
PLIST_TMPL   := service/$(PLIST_LABEL).plist.template
PLIST_LOCAL  := $(PLIST_LABEL).plist
PLIST_DEST   := $(HOME)/Library/LaunchAgents/$(PLIST_LOCAL)

.PHONY: build config install uninstall start stop restart status logs

build:
	go build -o $(BINARY) ./cmd/key-logger/
	codesign --sign - --force --identifier $(PLIST_LABEL) $(BINARY)

config:
	@if [ ! -f $(PLIST_LOCAL) ]; then \
		cp $(PLIST_TMPL) $(PLIST_LOCAL); \
		echo "Created $(PLIST_LOCAL) from template."; \
		echo "Edit it now to fill in your Loki URL, hostname, and any other flags."; \
	else \
		echo "$(PLIST_LOCAL) already exists, skipping copy."; \
	fi

install: build config
	@if grep -q 'LOKI_URL\|HOSTNAME' $(PLIST_LOCAL); then \
		echo "ERROR: $(PLIST_LOCAL) still contains placeholder values (LOKI_URL / HOSTNAME)."; \
		echo "Edit the file and replace them before installing."; \
		exit 1; \
	fi
	@UPGRADING=false; \
	if [ -f $(INSTALL_DIR)/$(BINARY) ]; then \
		UPGRADING=true; \
	fi; \
	sudo cp $(BINARY) $(INSTALL_DIR)/$(BINARY); \
	mkdir -p $(HOME)/Library/LaunchAgents; \
	cp $(PLIST_LOCAL) $(PLIST_DEST); \
	launchctl bootout gui/$$(id -u) $(PLIST_DEST) 2>/dev/null || true; \
	launchctl bootstrap gui/$$(id -u) $(PLIST_DEST); \
	echo "Installed and started $(PLIST_LABEL)."; \
	if [ "$$UPGRADING" = true ]; then \
		echo ""; \
		echo "NOTE: The binary was replaced. macOS may invalidate permissions even though"; \
		echo "System Settings still shows them as enabled. If the service fails to start:"; \
		echo "  1. Open System Settings > Privacy & Security"; \
		echo "  2. For both Accessibility and Screen Recording:"; \
		echo "     - Remove $(INSTALL_DIR)/$(BINARY) (click the minus button)"; \
		echo "     - Re-add it (click plus, navigate to $(INSTALL_DIR)/$(BINARY))"; \
		echo "  3. Run: make restart"; \
	else \
		echo "Grant Accessibility and Screen Recording permissions to $(INSTALL_DIR)/$(BINARY)"; \
		echo "in System Settings > Privacy & Security."; \
	fi

uninstall:
	launchctl bootout gui/$$(id -u) $(PLIST_DEST) 2>/dev/null || true
	rm -f $(PLIST_DEST)
	sudo rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Uninstalled $(PLIST_LABEL)."

start:
	launchctl bootstrap gui/$$(id -u) $(PLIST_DEST)
	@echo "Started $(PLIST_LABEL)."

stop:
	launchctl bootout gui/$$(id -u) $(PLIST_DEST)
	@echo "Stopped $(PLIST_LABEL)."

restart: stop start

status:
	@launchctl print gui/$$(id -u)/$(PLIST_LABEL) 2>/dev/null || echo "$(PLIST_LABEL) is not loaded."

logs:
	@tail -f /tmp/key-logger.stdout.log /tmp/key-logger.stderr.log
