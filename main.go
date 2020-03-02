/*
Copyright 2020 Will Thames, Skedulo

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
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
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
)

type PodProcessorApp struct {
	processor.App
	labels      map[string]struct{}
	namespaces  []string
	podCache    *PodCache
	tagPrefix   string
	podNameTags map[string]struct{}
}

func NewPodProcessorApp() *PodProcessorApp {
	a := new(PodProcessorApp)
	a.podCache = NewPodCache()
	a.Receiver = a
	labelValue := flag.String("labels", "", "comma-separated list of labels to use as tags (default is all)")
	namespaceValue := flag.String("namespaces", "", "comma-separated list of namespaces to watch (default is all)")
	tagPrefixValue := flag.String("tag-prefix", "", "prefix to insert in front of the tag name")
	podNameTagValue := flag.String("pod-name-tags", "pod_name", "comma-separated list of tags containing the pod name")

	a.BaseCLI()
	flag.Parse()

	if len(*labelValue) > 0 {
		labels := strings.Split(*labelValue, ",")
		a.labels = make(map[string]struct{}, len(labels))
		for _, label := range labels {
			a.labels[label] = struct{}{}
		}
	}
	if len(*podNameTagValue) > 0 {
		podNameTags := strings.Split(*podNameTagValue, ",")
		a.podNameTags = make(map[string]struct{}, len(podNameTags))
		for _, podNameTag := range podNameTags {
			a.podNameTags[podNameTag] = struct{}{}
		}
	}
	if len(*namespaceValue) > 0 {
		a.namespaces = strings.Split(*namespaceValue, ",")
	}
	a.tagPrefix = *tagPrefixValue
	return a
}

func (a *PodProcessorApp) ReceiveSpan(span *span.Span) {
	podName := ""
	for _, ba := range span.BinaryAnnotations {
		_, ok := a.podNameTags[ba.Key]
		if ok {
			podName = ba.Value.(string)
			break
		}
	}
	if podName == "" {
		a.writeSpan(span)
		return
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
			span.AddTag(a.tagPrefix+key, value)
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

func watcher(ctx context.Context, clientset *kubernetes.Clientset, podCache *PodCache, namespace string) {
	watchpods, err := clientset.CoreV1().Pods(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		switch statusErr := err.(type) {
		case *errors.StatusError:
			log.Fatalf("Could not watch pods (%#v): %s ", statusErr.ErrStatus.Details, statusErr.ErrStatus.Reason)
		default:
			log.Fatalf("Could not watch pods: %#v", err)
		}
	}
	for {
		select {
		case event := <-watchpods.ResultChan():
			p, ok := event.Object.(*v1.Pod)
			if !ok {
				logrus.WithField("event", event).Warn("Could not convert watch event into pod")
				continue
			}
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
		case <-ctx.Done():
			watchpods.Stop()
			return
		}
	}
}

func connectCluster() *kubernetes.Clientset {
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
	return clientset
}

func main() {
	a := NewPodProcessorApp()
	clientset := connectCluster()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if len(a.namespaces) > 0 {
		for _, namespace := range a.namespaces {
			go watcher(ctx, clientset, a.podCache, namespace)
		}
	} else {
		go watcher(ctx, clientset, a.podCache, "")
	}
	a.Serve()
	<-sigCh
}
