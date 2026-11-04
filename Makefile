# Release helpers. Usage:
#
#   make release VERSION=v0.2.5   # vet/test/build the CLI on main, fast-forward dev to main, create the annotated tag
#   make publish VERSION=v0.2.5   # release, then push main, dev and the tag in one atomic push (asks first)
#   make untag   VERSION=v0.2.5   # delete the local tag (before it is pushed)
#   make latest                     # print the latest release tag
#   make check                      # vet, test and build the CLI, as CI does
#   make sdk-docs-check             # check ai/sdk/go against the pinned gofi-sdk-go release, as CI does
#
# Only `make publish` pushes, and only after you confirm: the tag triggers
# .github/workflows/release.yml, the public release. `make release` stays
# local; push it by hand with: git push origin main dev <tag>
# YES=1 skips the confirmation (for a release you have already reviewed).

CLI    := cli
BRANCH := main
DEV    := dev
LATEST  = $(shell git tag -l 'v*' --sort=-v:refname | head -1)

.PHONY: release publish untag latest check check-version sdk-docs-check

check-version:
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "use: make <target> VERSION=vX.Y.Z (latest is $(LATEST))"; exit 1; }

check:
	cd $(CLI) && export GOWORK=off && go vet ./... && go test ./... && go build -o /dev/null ./cmd/gofi

# The gofi-sdk-go release the bundled Go docs describe (config.SDKGoVersion).
SDK_GO_VERSION = $(shell sed -n 's/^const SDKGoVersion = "\(v[0-9][0-9.]*\)"$$/\1/p' $(CLI)/internal/config/defaults.go)

sdk-docs-check:
	@test -n "$(SDK_GO_VERSION)" || { echo "SDKGoVersion not found in $(CLI)/internal/config/defaults.go"; exit 1; }
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	git clone -q --depth 1 --branch gofi/$(SDK_GO_VERSION) https://github.com/joaoprofile/gofi-sdk-go "$$tmp/sdk" && \
	cd $(CLI) && GOWORK=off GOFI_SDK_CHECKOUT="$$tmp/sdk" go test -count=1 -run 'TestBundledGoDocs' -v ./internal/sdkdoc/

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

# The branches and the tag go in one atomic push: either all land or none
# does, so the release never runs on a tag whose commits are not on main.
publish: release
	@branches="$(BRANCH)"; git show-ref -q --verify refs/heads/$(DEV) && branches="$$branches $(DEV)"; \
	echo "about to push $$branches and $(VERSION) to origin — the tag publishes the release"; \
	if [ "$(YES)" != "1" ]; then \
		printf "publish $(VERSION)? [y/N] "; read answer; \
		case "$$answer" in y|Y|yes|s|S|sim) ;; *) echo "not pushed: the tag $(VERSION) stays local (make untag VERSION=$(VERSION) to drop it)"; exit 1;; esac; \
	fi; \
	git push --atomic origin $$branches $(VERSION) || { echo "push failed: nothing was published; the tag $(VERSION) stays local — fix it and run: git push --atomic origin $$branches $(VERSION)"; exit 1; }; \
	echo "published $(VERSION): follow the release with gh run watch, or the Actions tab"

untag: check-version
	git tag -d $(VERSION)

latest:
	@echo $(LATEST)
