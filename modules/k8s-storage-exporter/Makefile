NAME      ?= debug
NAMESPACE ?= default
VERSION   ?= 0.0.0-debug

.PHONY: debug ls build lint

# render the manifests with plain cue (no timoni needed)
debug:
	cue cmd -t debug -t name=$(NAME) -t namespace=$(NAMESPACE) -t mv=$(VERSION) build

# list the resources the module produces
ls:
	cue cmd -t debug -t name=$(NAME) -t namespace=$(NAMESPACE) -t mv=$(VERSION) ls

# render the manifests with timoni
build:
	timoni build $(NAME) . --namespace $(NAMESPACE)

lint:
	timoni mod vet .
