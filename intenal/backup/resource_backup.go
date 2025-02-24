package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

func BackupResource(resourceName, namespace, directory string) error {
	config, err := clientcmd.BuildConfigFromKubeconfigGetter("", clientcmd.NewDefaultClientConfigLoadingRules().Load)
	if err != nil {
		return fmt.Errorf("error creating k8 client config: %w", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating k8 client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	_, rl, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return fmt.Errorf("error discovering api server resources: %w", err)
	}

	var grv schema.GroupVersionResource
	var found bool
	var namespaced bool

	for _, resource := range rl {
		for _, rr := range resource.APIResources {
			if rr.SingularName == resourceName {
				var version string
				var group string
				groupVersion := strings.Split(resource.GroupVersion, "/")
				if len(groupVersion) == 1 {
					version = groupVersion[0]
				} else {
					group = groupVersion[0]
					version = groupVersion[1]
				}
				grv = schema.GroupVersionResource{Group: group, Version: version, Resource: rr.Name}
				namespaced = rr.Namespaced
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("resource with name %s not found", resourceName)
	}

	if !namespaced {
		namespace = v1.NamespaceNone
	}

	resources, err := client.Resource(grv).Namespace(namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error list resource %s: %w", resourceName, err)
	}

	for _, item := range resources.Items {
		obj := item.Object
		removeStatus(obj)
		removeServerGeneratedFields(obj)
		specs, ok := obj["specs"].(map[string]interface{})
		if ok {
			removeNullsAndEmptyValues(specs)
		}

		fileName := path.Join(directory, item.GetName()+".yaml")

		f, err := os.OpenFile(fileName,
			os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to save %s %s: %w", resourceName, item.GetName(), err)
		}

		defer func() {
			if err := f.Close(); err != nil {
				log.Printf("error closing file %s: %s", fileName, err.Error())
			}
		}()

		enc := yaml.NewEncoder(f)
		enc.SetIndent(2)

		err = enc.Encode(obj)
		if err != nil {
			return fmt.Errorf("error encoding file: %w", err)
		}
	}

	return nil
}

func removeServerGeneratedFields(obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	delete(metadata, "selfLink")
	delete(metadata, "uid")
	delete(metadata, "resourceVersion")
	delete(metadata, "generation")
	delete(metadata, "creationTimestamp")
	delete(metadata, "deletionTimestamp")
	delete(metadata, "deletionGracePeriodSeconds")
	delete(metadata, "managedFields")
}

func removeStatus(obj map[string]interface{}) {
	delete(obj, "status")
}

func removeNullsAndEmptyValues(root map[string]interface{}) {
	for k, v := range root {
		if k == "emptyDir" {
			continue
		}
		obj, ok := v.(map[string]interface{})
		if ok {
			if len(obj) == 0 {
				delete(root, k)
			} else {
				removeNullsAndEmptyValues(obj)
			}
			continue
		}

		array, ok := v.([]interface{})
		if ok {
			if array == nil {
				delete(root, k)
			} else {
				for _, vi := range array {
					arrayItem, ok := vi.(map[string]interface{})
					if ok {
						removeNullsAndEmptyValues(arrayItem)
					}
				}
			}
			continue
		}

		if v == nil {
			delete(root, k)
		}
	}
}
