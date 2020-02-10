VERSION = $(shell git describe --long)

image:
	docker build -t opentracing-pod-annotator:$(VERSION) .

push: image
	docker tag opentracing-pod-annotator:$(VERSION) quay.io/skedulo/opentracing-pod-annotator:$(VERSION)
