package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type getConfigFunc func() (*rest.Config, error)
type getDynamicClientFunc func(*rest.Config) (dynamic.Interface, error)
type getDiscoveryClientFunc func(*rest.Config) (discovery.DiscoveryInterface, error)
type openFileFunc func(fileAbsolutePath string) (io.WriteCloser, error)

var defaultGetConfig getConfigFunc = func() (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", clientcmd.NewDefaultClientConfigLoadingRules().Load)
}

var defaultGetDynamicClientFunc getDynamicClientFunc = func(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

var defaultGetDiscoveryClientFunc getDiscoveryClientFunc = func(config *rest.Config) (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(config)
}

var defaultOpenFileFunc openFileFunc = func(fileAbsolutePath string) (io.WriteCloser, error) {
	return os.OpenFile(fileAbsolutePath,
		os.O_RDWR|os.O_CREATE, 0644)
}

func BackupResource(resourceKind, namespace, directory string, archive bool) error {
	return backupResource(resourceKind, namespace, directory, archive, defaultGetConfig,
		defaultGetDynamicClientFunc, defaultGetDiscoveryClientFunc, defaultOpenFileFunc)
}

func backupResource(resourceKind, namespace, directory string, archive bool, getConfigFunc getConfigFunc,
	getDynamicClientFunc getDynamicClientFunc, getDiscoveryClient getDiscoveryClientFunc, openfileFunc openFileFunc) error {
	config, err := getConfigFunc()
	if err != nil {
		return fmt.Errorf("error creating k8 client config: %w", err)
	}

	discoveryClient, err := getDiscoveryClient(config)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	_, sgr, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return fmt.Errorf("error discovering api server resources: %w", err)
	}

	var grv schema.GroupVersionResource
	var found bool
	var namespaced bool

	for _, resource := range sgr {
		for _, ar := range resource.APIResources {
			if ar.SingularName == resourceKind {
				var version string
				var group string
				groupVersion := strings.Split(resource.GroupVersion, "/")
				if len(groupVersion) == 1 {
					version = groupVersion[0]
				} else {
					group = groupVersion[0]
					version = groupVersion[1]
				}
				grv = schema.GroupVersionResource{Group: group, Version: version, Resource: ar.Name}
				namespaced = ar.Namespaced
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("resource with name %s not found", resourceKind)
	}

	if !namespaced {
		namespace = v1.NamespaceNone
	}

	client, err := getDynamicClientFunc(config)
	if err != nil {
		return fmt.Errorf("error creating k8 client: %w", err)
	}

	resources, err := client.Resource(grv).Namespace(namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing resource %s: %w", resourceKind, err)
	}

	var zipWriter *zip.Writer

	if archive {
		var archiveFileName string
		if namespaced {
			archiveFileName = fmt.Sprintf("%s_%s.zip", resourceKind, namespace)
		} else {
			archiveFileName = fmt.Sprintf("%s.zip", resourceKind)
		}
		archiveAbsolutePath := path.Join(directory, archiveFileName)

		archiveFile, err := openfileFunc(archiveAbsolutePath)
		if err != nil {
			return fmt.Errorf("error creating archive file %s: %w", archiveFileName, err)
		}
		zipWriter = zip.NewWriter(archiveFile)
		defer func() {
			if err := zipWriter.Close(); err != nil {
				log.Printf("error closing zip writer: %s", err.Error())
			}
			if err := archiveFile.Close(); err != nil {
				log.Printf("error closing zip file: %s", err.Error())
			}
		}()
	}

	for _, item := range resources.Items {
		obj := item.Object
		removeStatus(obj)
		removeServerGeneratedFields(obj)
		specs, ok := obj["spec"].(map[string]interface{})
		if ok {
			removeNullValues(specs)
		}

		var fileName string
		if namespaced {
			fileName = fmt.Sprintf("%s_%s_%s.yaml", item.GetName(), resourceKind, namespace)
		} else {
			fileName = fmt.Sprintf("%s_%s.yaml", item.GetName(), resourceKind)
		}

		fileAbsolutePath := path.Join(directory, fileName)

		var f io.WriteCloser
		var currentZipWriter io.Writer
		var enc *yaml.Encoder

		if archive {
			currentZipWriter, err = zipWriter.Create(fileName)
			if err != nil {
				return fmt.Errorf("failed to add file %s to zip archive: %w", fileName, err)
			}
			enc = yaml.NewEncoder(currentZipWriter)
		} else {
			f, err = openfileFunc(fileAbsolutePath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", fileName, err)
			}
			enc = yaml.NewEncoder(f)
			defer func() {
				if err := f.Close(); err != nil {
					log.Printf("error closing file %s: %s", fileAbsolutePath, err.Error())
				}
			}()
		}

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

func removeNullValues(root map[string]interface{}) {
	for k, v := range root {
		obj, ok := v.(map[string]interface{})
		if ok {
			// if len(obj) == 0 {
			// we can try to remove fields that have empty object as a value
			// like securityContext: {}
			// but there are always exceptions like emptyDir: {}
			// it's difficult to know all of them in advance
			// e.g a cluster may contain custom CRDs
			if obj == nil {
				delete(root, k)
			} else {
				removeNullValues(obj)
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
						removeNullValues(arrayItem)
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
