# Release helpers. Usage:
#
#   make release VERSION=v0.2.5   # vet/test/build the CLI on main, fast-forward dev to main, create the annotated tag
#   make untag   VERSION=v0.2.5   # delete the local tag (before it is pushed)
#   make latest                     # print the latest release tag
#   make check                      # vet, test and build the CLI, as CI does
#
# Nothing here pushes. Push by hand: git push origin main dev <tag>
# (the tag triggers .github/workflows/release.yml).

CLI    := cli
BRANCH := main
DEV    := dev
LATEST  = $(shell git tag -l 'v*' --sort=-v:refname | head -1)

.PHONY: release untag latest check check-version

check-version:
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "use: make <target> VERSION=vX.Y.Z (latest is $(LATEST))"; exit 1; }

check:
	cd $(CLI) && export GOWORK=off && go vet ./... && go test ./... && go build -o /dev/null ./cmd/gofi

release: check-version
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "$(BRANCH)" || { echo "checkout $(BRANCH) first"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "working tree is not clean: commit or stash first"; exit 1; }
	@test -z "$$(git tag -l $(VERSION))" || { echo "$(VERSION) already exists, latest is $(LATEST)"; exit 1; }
	git fetch -q origin $(BRANCH)
	@test -z "$$(git rev-list HEAD..origin/$(BRANCH))" || { echo "$(BRANCH) is behind origin/$(BRANCH): pull first"; exit 1; }
	@$(MAKE) -s check
	@if git show-ref -q --verify refs/heads/$(DEV); then \
		git merge-base --is-ancestor $(DEV) $(BRANCH) || { echo "$(DEV) has commits not in $(BRANCH): merge them first"; exit 1; }; \
	fi
	git branch -f $(DEV) $(BRANCH)
	git tag -a $(VERSION) -m "release $(VERSION)"
	@echo "done: tag $(VERSION) on $$(git rev-parse --short HEAD). Next: git push origin $(BRANCH) $(DEV) $(VERSION)"

untag: check-version
	git tag -d $(VERSION)

latest:
	@echo $(LATEST)
