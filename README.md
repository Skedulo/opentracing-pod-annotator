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

## Usage

```
Usage of /go/bin/opentracing-annotator:
  -collector-url string
        Host to forward traces. Not setting this will work as dry run
  -labels string
        comma-separated list of labels to use as tags (default is all)
  -log-level string
        log level (default "Info")
  -metrics-port int
        prometheus /metrics port (default 10010)
  -namespaces string
        comma-separated list of namespaces to watch (default is all)
  -pod-name-tags string
        comma-separated list of tags containing the pod name (default "pod_name")
  -port int
        server port (default 8080)
  -tag-prefix string
        prefix to insert in front of the tag name
```

## Deployment

Currently this needs to run inside Kubernetes with appropriate RBAC
permissions.

`deploy/annotator.yml` is likely to be a reasonable starting point, but you may
want to use RoleBindings to restrict the permissions to specific namespaces.
