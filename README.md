# opentracing-pod-annotator

The opentracing-pod-annotator listens for Zipkin v1 spans (thrift or json).
For each span, it reads the `pod_name` annotation, looks up the pod and then
adds a set of labels (defined by the `--labels` argument, or all by default)
as binary annotations and forwards the response on to a collector (if set,
otherwise it logs the amended spans to stdout)

By default it looks up pods in all namespaces which might cause permissions
issues or performance issues in very large environments, so namespaces can
be limited to a subset using the `--namespaces` argument. This does mean that
the annotator might receive spans from pods that it can't annotate - so namespaced
annotators might make sense in that scenario.
