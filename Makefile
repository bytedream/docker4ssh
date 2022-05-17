VERSION=0.1.2

BUILDDIR = .
_BUILDDIR = $(shell realpath $(BUILDDIR))/

build: build-server build-container build-extra

build-server:
	cd server/ && go build -o $(_BUILDDIR)/docker4ssh

build-container: DEBUG=false
build-container:
	@if $(DEBUG); then\
		cd container/ && cargo build --target x86_64-unknown-linux-musl --target-dir $(_BUILDDIR) --bin configure;\
	else\
		cd container/ && cargo build --target x86_64-unknown-linux-musl --target-dir $(_BUILDDIR) --release --bin configure;\
	fi
	cp -rf $(_BUILDDIR)/x86_64-unknown-linux-musl/$(shell if $(DEBUG); then echo debug; else echo release; fi)/configure $(_BUILDDIR)

build-extra: SSHPASS:=$(shell LC_ALL=C tr -dc 'A-Za-z0-9!#$%&()*,-.:<=>?@[]^_{}~' < /dev/urandom | head -c 18 ; echo)
build-extra:
	@if [ "$(_BUILDDIR)" != "$(shell realpath .)/" ]; then\
		cp -rf LICENSE $(_BUILDDIR)/LICENSE;\
		cp -rf man/ $(_BUILDDIR);\
	fi
	yes | ssh-keygen -t ed25519 -f $(_BUILDDIR)/docker4ssh.key -N "$(SSHPASS)" -b 4096 > /dev/null
	cp -rf extra/docker4ssh.conf $(_BUILDDIR)
	sed -i "s|Passphrase = \"\"|Passphrase = \"$(SSHPASS)\"|" $(_BUILDDIR)/docker4ssh.conf
	cat extra/database.sql | sqlite3 $(_BUILDDIR)/docker4ssh.sqlite3
	mkdir -p $(_BUILDDIR)/profile/ && cp -f extra/profile.conf $(_BUILDDIR)/profile/

optimize: optimize-server optimize-container

optimize-server:
	strip $(_BUILDDIR)/docker4ssh

optimize-container:
	strip $(_BUILDDIR)/configure

clean: clean-server clean-container clean-extra

clean-server:
	rm -rf $(_BUILDDIR)/docker4ssh

clean-container:
	rm -rf $(_BUILDDIR)/{x86_64-unknown-linux-musl,configure}

clean-extra:
	rm -rf $(_BUILDDIR)/docker4ssh*
	rm -rf $(_BUILDDIR)/man/
	rm -rf $(_BUILDDIR)/profile/

DESTDIR=
PREFIX=/usr
install:
	install -Dm755 $(_BUILDDIR)docker4ssh $(DESTDIR)$(PREFIX)/bin/docker4ssh
	install -Dm644 $(_BUILDDIR)LICENSE $(DESTDIR)$(PREFIX)/share/licenses/docker4ssh/LICENSE
	install -Dm644 $(_BUILDDIR)man/docker4ssh.1 $(DESTDIR)$(PREFIX)/share/man/man1/docker4ssh.1
	install -Dm644 $(_BUILDDIR)man/docker4ssh.conf.5 $(DESTDIR)$(PREFIX)/share/man/man5/docker4ssh.conf.5
	install -Dm644 $(_BUILDDIR)man/profile.conf.5 $(DESTDIR)$(PREFIX)/share/man/man5/profile.conf.5

	install -Dm755 $(_BUILDDIR)configure $(DESTDIR)/etc/docker4ssh/configure
	install -Dm775 $(_BUILDDIR)docker4ssh.conf $(DESTDIR)/etc/docker4ssh/docker4ssh.conf
	install -Dm755 $(_BUILDDIR)docker4ssh.sqlite3 $(DESTDIR)/etc/docker4ssh/docker4ssh.sqlite3
	install -Dm755 $(_BUILDDIR)docker4ssh.key $(DESTDIR)/etc/docker4ssh/docker4ssh.key
	install -Dm644 $(_BUILDDIR)man/* -t $(DESTDIR)/etc/docker4ssh/man/
	install -Dm644 $(_BUILDDIR)profile/* -t $(DESTDIR)/etc/docker4ssh/profile/
	install -Dm644 $(_BUILDDIR)LICENSE $(DESTDIR)/etc/docker4ssh/LICENSE

	touch $(DESTDIR)/etc/docker4ssh/docker4ssh.log && chmod 777 $(DESTDIR)/etc/docker4ssh/docker4ssh.log

uninstall:
	rm -rf $(DESTDIR)/etc/docker4ssh/
	rm -f $(DESTDIR)$(PREFIX)/bin/docker4ssh
	rm -f $(DESTDIR)$(PREFIX)/share/man/man1/docker4ssh.1
	rm -f $(DESTDIR)$(PREFIX)/share/man/man5/{docker4ssh,profile}.5
	rm -f $(DESTDIR)$(PREFIX)/share/licenses/docker4ssh/LICENSE

release:
	mkdir -p /tmp/docker4ssh-$(VERSION)-build/ /tmp/docker4ssh-$(VERSION)-release/
	$(MAKE) BUILDDIR=/tmp/docker4ssh-$(VERSION)-build/ SSHPASS= build optimize
	$(MAKE) BUILDDIR=/tmp/docker4ssh-$(VERSION)-build/ DESTDIR=/tmp/docker4ssh-$(VERSION)-release/ install
	tar -C /tmp/docker4ssh-$(VERSION)-release/ -czf docker4ssh-$(VERSION).tar.gz .

RUNDIR=/tmp/docker4ssh

.PHONY run:
run:
	$(MAKE) BUILDDIR=$(RUNDIR) SSHPASS= build
	cd $(RUNDIR) && ./docker4ssh

develop: SERVERSUM = $(shell find server/ -type f -exec md5sum {} + | LC_ALL=C sort | md5sum | cut -d ' ' -f1)
develop: CONTAINERSUM = $(shell find container/src/ -type f -exec md5sum {} + | LC_ALL=C sort | md5sum | cut -d ' ' -f1)
develop: EXTRASUM = $(shell find extra/ -type f -exec md5sum {} + | LC_ALL=C sort | md5sum | cut -d ' ' -f1)
# there is maybe a better way to do this stuff but for the moment this works out
develop:
	@if [ ! -d $(RUNDIR) ]; then\
		$(MAKE) BUILDDIR=$(RUNDIR) DEBUG=true SSHPASS= build;\
		if [[ $$? -ne 0 ]]; then exit 2; fi;\
		echo -n $(SERVER) > $(RUNDIR)/SERVERSUM;\
		echo -n $(CLIENTSUM) > $(RUNDIR)/CONTAINERSUM;\
		echo -n $(EXTRASUM) > $(RUNDIR)/EXTRASUM;\
	else\
		if [ "$(shell cat $(RUNDIR)/SERVERSUM)" != "$(SERVERSUM)" ]; then\
			$(MAKE) BUILDDIR=$(RUNDIR) clean-server;\
			$(MAKE) BUILDDIR=$(RUNDIR) build-server;\
			if [[ $$? -ne 0 ]]; then exit 2; fi;\
			echo -n $(SERVERSUM) > $(RUNDIR)/SERVERSUM;\
		fi;\
		if [ "$(shell cat $(RUNDIR)/CONTAINERSUM)" != "$(CONTAINERSUM)" ]; then\
			$(MAKE) BUILDDIR=$(RUNDIR) clean-container;\
			$(MAKE) BUILDDIR=$(RUNDIR) DEBUG=true build-container;\
			if [[ $$? -ne 0 ]]; then exit 2; fi;\
			echo -n $(CONTAINERSUM) > $(RUNDIR)/CONTAINERSUM;\
		fi;\
		if [ "$(shell cat $(RUNDIR)/EXTRASUM)" != "$(EXTRASUM)" ]; then\
			$(MAKE) BUILDDIR=$(RUNDIR) clean-extra;\
			$(MAKE) BUILDDIR=$(RUNDIR) SSHPASS= build-extra;\
			if [[ $$? -ne 0 ]]; then exit 2; fi;\
			echo -n $(EXTRASUM) > $(RUNDIR)/EXTRASUM;\
		fi;\
	fi
	cd $(RUNDIR) && LOGGING_LEVEL="debug" ./docker4ssh start
