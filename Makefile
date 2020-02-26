VERSION = $(shell git describe --long)

image:
	docker build -t quay.io/skedulo/opentracing-pod-annotator:$(VERSION) .

push: image
	docker push quay.io/skedulo/opentracing-pod-annotator:$(VERSION)
