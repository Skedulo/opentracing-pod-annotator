/*
Copyright 2016 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"log"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/willthames/opentracing-processor/processor"
	"github.com/willthames/opentracing-processor/span"
	v1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

type PodProcessorApp struct {
	processor.App
	labels     map[string]bool
	namespaces []string
	podCache   *PodCache
}

func NewPodProcessorApp() *PodProcessorApp {
	a := new(PodProcessorApp)
	a.podCache = NewPodCache()
	a.Receiver = a
	labelValue := flag.String("labels", "", "comma-separated list of labels to use as tags (default is all)")
	namespaceValue := flag.String("namespaces", "", "comma-separated list of namespaces to watch (default is all)")

	a.BaseCLI()
	flag.Parse()

	if len(*labelValue) > 0 {
		labels := strings.Split(*labelValue, ",")
		if len(labels) > 0 {
			a.labels = make(map[string]bool, len(labels))
			for _, label := range labels {
				a.labels[label] = true
			}
		}
	}
	if len(*namespaceValue) > 0 {
		a.namespaces = strings.Split(*namespaceValue, ",")
	}
	return a
}

func (a *PodProcessorApp) ReceiveSpan(span *span.Span) {
	podName := ""
	for _, ba := range span.BinaryAnnotations {
		if ba.Key == "pod_name" {
			podName = ba.Value.(string)
			break
		}
	}
	for _, ann := range span.Annotations {
		logrus.WithField("annotation", ann).Debug("Annotation")
		if podName != "" {
			if ann.Host != nil {
				podName = ann.Host.ServiceName
			}
		}
	}
	pod, ok := a.podCache.Get(podName)
	if ok {
		for key, value := range pod.ObjectMeta.Labels {
			if len(a.labels) != 0 {
				_, ok = a.labels[key]
				if !ok {
					continue
				}
			}
			span.AddTag(key, value)
		}
	} else {
		logrus.WithField("pod", podName).Info("span received for pod not in cache")
	}
	a.writeSpan(span)
}

func (a *PodProcessorApp) writeSpan(wspan *span.Span) error {
	spans := []*span.Span{wspan}
	body, err := json.Marshal(spans)
	if err != nil {
		logrus.WithError(err).WithField("span", wspan).Error("Error converting span to JSON")
		return err
	}
	if a.Forwarder != nil {
		if err := a.Forwarder.Send(processor.Payload{ContentType: "application/json", Body: body}); err != nil {
			logrus.WithError(err).Error("Error forwarding trace")
			logrus.WithField("body", body).Debug("Error forwarding trace body")
			return err
		}
		logrus.WithField("span", wspan).Debug("accepting span")
	} else {
		logrus.WithField("span", wspan).Info("dry-run: would have forwarded span")
	}
	return nil
}

func watcher(clientset *kubernetes.Clientset, podCache *PodCache, namespace string) {
	watchpods, err := clientset.CoreV1().Pods(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		switch statusErr := err.(type) {
		case *errors.StatusError:
			log.Fatalf("Could not watch pods (%#v): %s ", statusErr.ErrStatus.Details, statusErr.ErrStatus.Reason)
		default:
			log.Fatalf("Could not watch pods: %#v", err)
		}
	}
	for event := range watchpods.ResultChan() {
		p := event.Object.(*v1.Pod)
		switch event.Type {
		case watch.Added:
			existing, ok := podCache.Get(p.ObjectMeta.Name)
			if ok && p.ObjectMeta.Namespace != existing.ObjectMeta.Namespace {
				logrus.WithField("name", existing.ObjectMeta.Name).WithField("namespace", existing.ObjectMeta.Namespace).Warn("Pod already exists in cache")
			}
			podCache.Set(p.ObjectMeta.Name, p)
			logrus.WithField("namespace", p.ObjectMeta.Namespace).WithField("name", p.ObjectMeta.Name).Debug("Added pod to cache")
		case watch.Deleted:
			podCache.Delete(p.ObjectMeta.Name)
			logrus.WithField("namespace", p.ObjectMeta.Namespace).WithField("name", p.ObjectMeta.Name).Debug("Deleted pod from cache")
		}
	}
}

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	a := NewPodProcessorApp()
	if len(a.namespaces) > 0 {
		for _, namespace := range a.namespaces {
			go watcher(clientset, a.podCache, namespace)
		}
	} else {
		go watcher(clientset, a.podCache, "")
	}
	a.Serve()
}
