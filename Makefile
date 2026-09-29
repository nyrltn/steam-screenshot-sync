NAME    := steam-screenshot-sync
# The spec is the single source of truth for the version.
VERSION := $(shell sed -n 's/^Version:[[:space:]]*//p' $(NAME).spec)
DIST    := $(CURDIR)/dist
TARBALL := $(DIST)/$(NAME)-$(VERSION).tar.gz

USER_BIN   := $(HOME)/.local/bin
USER_UNITS := $(HOME)/.config/systemd/user

.PHONY: build test vet tarball rpm install-user uninstall-user clean

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(NAME) .

test:
	go test ./...

vet:
	go vet ./...

# Packs the working tree (committed or not) so rpm builds work before the first commit.
tarball:
	mkdir -p $(DIST)
	tar --exclude-vcs --exclude=./dist --exclude=./$(NAME) \
	    --transform 's,^\.,$(NAME)-$(VERSION),' \
	    -czf $(TARBALL) .

rpm: tarball
	rpmbuild -ba \
	    --define "_topdir $(DIST)/rpmbuild" \
	    --define "_sourcedir $(DIST)" \
	    $(NAME).spec
	@find $(DIST)/rpmbuild/RPMS $(DIST)/rpmbuild/SRPMS -name '*.rpm'

# Development install without the rpm: binary in ~/.local/bin, units in ~/.config/systemd/user.
install-user: build
	install -Dm0755 $(NAME) $(USER_BIN)/$(NAME)
	install -d $(USER_UNITS)
	sed 's,^ExecStart=/usr/bin/,ExecStart=%h/.local/bin/,' systemd/$(NAME).service > $(USER_UNITS)/$(NAME).service
	install -m0644 systemd/$(NAME).timer $(USER_UNITS)/$(NAME).timer
	systemctl --user daemon-reload
	systemctl --user enable --now $(NAME).timer

uninstall-user:
	-systemctl --user disable --now $(NAME).timer
	rm -f $(USER_BIN)/$(NAME) $(USER_UNITS)/$(NAME).service $(USER_UNITS)/$(NAME).timer
	systemctl --user daemon-reload

clean:
	rm -rf $(NAME) $(DIST)
