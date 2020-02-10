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

// Note: the example only works with the code within the same release/branch.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
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

func watcher(clientset *kubernetes.Clientset, watchpods watch.Interface, podCache *PodCache) {
	for event := range watchpods.ResultChan() {
		p := event.Object.(*v1.Pod)
		switch event.Type {
		case watch.Added:
			podCache.Set(p.ObjectMeta.Namespace, p.ObjectMeta.Name, p)
			logrus.WithField("namespace", p.ObjectMeta.Namespace).WithField("name", p.ObjectMeta.Name).Debug("Added pod to cache")
		case watch.Deleted:
			podCache.Delete(p.ObjectMeta.Namespace, p.ObjectMeta.Name)
			logrus.WithField("namespace", p.ObjectMeta.Namespace).WithField("name", p.ObjectMeta.Name).Debug("Deleted pod from cache")
		}
	}
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)
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
	watchpods, err := clientset.CoreV1().Pods("").Watch(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}

	podCache := NewPodCache()
	go watcher(clientset, watchpods, podCache)

	mux := http.NewServeMux()
	mux.HandleFunc("/pod/", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		parts := strings.Split(url, "/")
		if len(parts) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("%s not of form /pod/namespace/name", url)))
		} else {
			name := parts[3]
			namespace := parts[2]
			logrus.WithField("name", name).WithField("namespace", namespace).Debug("Looking up pod")
			pod, ok := podCache.Get(namespace, name)
			if ok {

				out, err := json.Marshal(pod.ObjectMeta.Labels)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					logrus.WithError(err).Warn("Could not convert pod labels to JSON")
				} else {
					w.WriteHeader(http.StatusOK)
					w.Write(out)
				}
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", 8080),
		Handler: mux,
	}
	server.ListenAndServe()
}
