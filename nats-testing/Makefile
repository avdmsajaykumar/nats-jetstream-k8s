REGISTRY ?= harbor.registry.prod.cdsf.io/keycore
docker:
ifdef IMAGE_VERSION
	docker build -t ${REGISTRY}/nats/nats-testing:${IMAGE_VERSION} .
else
	$(error IMAGE_VERSION is undefined.)
endif