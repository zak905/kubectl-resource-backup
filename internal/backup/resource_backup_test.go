package backup

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	fakek8 "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
)

const (
	testResourceName          = "unittest"
	testResourceName2         = "unittest2"
	testResourceKindPlural    = "backups"
	testResourceKind          = "Backup"
	testResourceKindLowerCase = "backup"
	testResourceKindList      = "BackupList"
	testResourceGV            = "restore/v1alpha1"
	testResourceGroup         = "restore"
	testResourceVersion       = "v1alpha1"
	testNamespace             = "namespace"
)

type getDiscoveryClientFuncFactory func(namespaced bool) getDiscoveryClientFunc
type getDynamicClientFuncFactory func(...runtime.Object) getDynamicClientFunc

var (
	globalObj = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": testResourceGV,
			"kind":       testResourceKind,
			"metadata": map[string]interface{}{
				"name": testResourceName,
				// all the server generated fields
				// should be removed as well
				"selfLink":                   "some self link",
				"uid":                        "ef9ceee0-2dca-11f0-be5c-74563c92ac72",
				"resourceVersion":            "1",
				"generation":                 "2",
				"creationTimestamp":          "anything",
				"deletionTimestamp":          "anything",
				"deletionGracePeriodSeconds": "2",
				"managedFields":              map[string]interface{}{"field1": "field2"},
			},
			"spec": map[string]interface{}{
				"field1": "this field should stay",
				// this field should be removed
				"filed2": nil,
			},
			"status": map[string]interface{}{
				"observedGeneration": "2",
			},
		},
	}

	obj = globalObj.DeepCopy()
	// add namespace
	_ = unstructured.SetNestedField(obj.Object, testNamespace, "metadata", "namespace")

	obj2 = obj.DeepCopy()
	// add namespace
	_ = unstructured.SetNestedField(obj2.Object, testResourceName2, "metadata", "name")

	globalObjAfterBackup = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": testResourceGV,
			"kind":       testResourceKind,
			"metadata": map[string]interface{}{
				"name": testResourceName,
			},
			"spec": map[string]interface{}{
				"field1": "this field should stay",
			},
		},
	}

	objAfterBackup = globalObjAfterBackup.DeepCopy()
	_              = unstructured.SetNestedField(objAfterBackup.Object, testNamespace, "metadata", "namespace")

	objAfterBackup2 = objAfterBackup.DeepCopy()
	_               = unstructured.SetNestedField(objAfterBackup2.Object, testResourceName2, "metadata", "name")

	okGetConfig getConfigFunc = func() (*rest.Config, error) {
		return nil, nil
	}

	okGetDynamicClientFuncFactory getDynamicClientFuncFactory = func(objects ...runtime.Object) getDynamicClientFunc {
		return func(_ *rest.Config) (dynamic.Interface, error) {
			dynamicClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
				map[schema.GroupVersionResource]string{
					{Group: testResourceGroup,
						Version: testResourceVersion, Resource: testResourceKindPlural}: testResourceKindList,
				},
				objects...,
			)
			return dynamicClient, nil
		}
	}

	okGetDiscoveryFuncFactory getDiscoveryClientFuncFactory = func(namespaced bool) getDiscoveryClientFunc {
		return func(_ *rest.Config) (discovery.DiscoveryInterface, error) {
			clientSet := fakek8.NewClientset()
			discoveryClient := clientSet.Discovery().(*fakediscovery.FakeDiscovery)

			discoveryClient.Resources = []*v1.APIResourceList{
				{
					GroupVersion: testResourceGV,
					APIResources: []v1.APIResource{
						{
							Name:         testResourceKindPlural,
							Namespaced:   namespaced,
							SingularName: testResourceKindLowerCase,
						},
					},
				},
			}

			return discoveryClient, nil
		}
	}

	errOp = errors.New("something happened")
)

func TestRemoveEmptyFields(t *testing.T) {
	for _, tc := range []int{1, 2, 3} {
		filename := fmt.Sprintf("manifest%d_input.yaml", tc)
		t.Run(filename, func(t *testing.T) {
			inputFileName := path.Join("test_resources", filename)
			b, err := os.ReadFile(inputFileName)
			if err != nil {
				t.Fatal(err.Error())
			}

			var manifest map[string]interface{}

			if err := yaml.Unmarshal(b, &manifest); err != nil {
				t.Fatal(err.Error())
			}

			removeNullValues(manifest)

			resultBuffer := bytes.Buffer{}

			encoder := yaml.NewEncoder(&resultBuffer)
			encoder.SetIndent(2)

			if err := encoder.Encode(manifest); err != nil {
				t.Fatal(err.Error())
			}

			expectedResultFileName := path.Join("test_resources", fmt.Sprintf("manifest%d_expected.yaml", tc))
			expectedBytes, err := os.ReadFile(expectedResultFileName)
			if err != nil {
				t.Fatal(err.Error())
			}

			assert.Equal(t, string(expectedBytes), resultBuffer.String())
		})
	}
}

type args struct {
	resourceKind                  string
	namespace                     string
	getConfigFunc                 getConfigFunc
	getDynamicClientFunc          getDynamicClientFuncFactory
	getDiscoveryClientFuncFactory getDiscoveryClientFuncFactory
	openFileFunc                  openFileFunc
}

type testCase struct {
	name       string
	args       args
	wantErr    bool
	errMsg     string
	listResult []runtime.Object
	expected   []*unstructured.Unstructured
}

func TestBackupResource(t *testing.T) {
	tests := []testCase{
		{
			name: "error getting config",
			args: args{
				getConfigFunc: func() (*rest.Config, error) {
					return nil, errOp
				},
			},
			wantErr: true,
			errMsg:  "error creating k8 client config: something happened",
		},
		{
			name: "error getting discovery client",
			args: args{
				getConfigFunc: okGetConfig,
				getDiscoveryClientFuncFactory: func(namespaced bool) getDiscoveryClientFunc {
					return func(c *rest.Config) (discovery.DiscoveryInterface, error) {
						return nil, errOp
					}
				},
			},
			wantErr: true,
			errMsg:  "error creating discovery client: something happened",
		},
		{
			name: "error getting ServerGroupsAndResources",
			args: args{
				resourceKind:  testResourceKindLowerCase,
				getConfigFunc: okGetConfig,
				getDiscoveryClientFuncFactory: func(namespaced bool) getDiscoveryClientFunc {
					return func(c *rest.Config) (discovery.DiscoveryInterface, error) {
						clientSet := fakek8.NewClientset()
						discoveryClient := clientSet.Discovery().(*fakediscovery.FakeDiscovery)

						discoveryClient.PrependReactor("*", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
							return true, nil, errOp
						})

						return discoveryClient, nil
					}
				},
			},
			wantErr: true,
			errMsg:  "error discovering api server resources: something happened",
		},
		{
			name: "resource not found",
			args: args{
				resourceKind:  testResourceKindLowerCase,
				getConfigFunc: okGetConfig,
				getDiscoveryClientFuncFactory: func(namespaced bool) getDiscoveryClientFunc {
					return func(c *rest.Config) (discovery.DiscoveryInterface, error) {
						clientSet := fakek8.NewClientset()
						discoveryClient := clientSet.Discovery().(*fakediscovery.FakeDiscovery)

						discoveryClient.Resources = []*v1.APIResourceList{}

						return discoveryClient, nil
					}
				},
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("resource with name %s not found", testResourceKindLowerCase),
		},
		{
			name: "error getting dynamic client",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc: func(o ...runtime.Object) getDynamicClientFunc {
					return func(c *rest.Config) (dynamic.Interface, error) {
						return nil, errOp
					}
				},
			},
			wantErr: true,
			errMsg:  "error creating k8 client: something happened",
		},
		{
			name: "error listing resources",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc: func(o ...runtime.Object) getDynamicClientFunc {
					return func(c *rest.Config) (dynamic.Interface, error) {
						dynamicClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
							map[schema.GroupVersionResource]string{
								{Group: testResourceGroup,
									Version: testResourceVersion, Resource: testResourceKindPlural}: testResourceKindList,
							},
						)
						dynamicClient.PrependReactor("*", "*", func(action kubetesting.Action,
						) (handled bool, ret runtime.Object, err error) {
							return true, nil, errOp
						})
						return dynamicClient, nil
					}
				},
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("error listing resource %s: something happened", testResourceKindLowerCase),
		},
		{
			name: "error opening file",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc:          okGetDynamicClientFuncFactory,
				namespace:                     testNamespace,
				openFileFunc: func(fileAbsolutePath string) (io.WriteCloser, error) {
					return nil, errOp
				},
			},
			wantErr: true,
			errMsg: fmt.Sprintf("failed to create file %s_%s_%s.yaml: something happened",
				testResourceName, testResourceKindLowerCase, testNamespace),
		},
		{
			name: "success",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				namespace:                     testNamespace,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc:          okGetDynamicClientFuncFactory,
				openFileFunc:                  defaultOpenFileFunc,
			},
			wantErr:    false,
			listResult: []runtime.Object{obj},
			expected:   []*unstructured.Unstructured{objAfterBackup},
		},
		{
			name: "success - global",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				namespace:                     v1.NamespaceNone,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc:          okGetDynamicClientFuncFactory,
				openFileFunc:                  defaultOpenFileFunc,
			},
			wantErr:    false,
			listResult: []runtime.Object{globalObj},
			expected:   []*unstructured.Unstructured{globalObjAfterBackup},
		},
		{
			name: "success - multiple objects",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				namespace:                     v1.NamespaceNone,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc:          okGetDynamicClientFuncFactory,
				openFileFunc:                  defaultOpenFileFunc,
			},
			wantErr:    false,
			listResult: []runtime.Object{obj, obj2},
			expected:   []*unstructured.Unstructured{objAfterBackup, objAfterBackup2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testDir string
			var err error
			if tt.args.openFileFunc != nil && !tt.wantErr {
				testDir, err = os.MkdirTemp("", "unittest")
				t.Cleanup(func() {
					if err := os.RemoveAll(testDir); err != nil {
						t.Log(err.Error())
					}
				})
				assert.NoError(t, err)
			}
			var getDicoveryClientFunc getDiscoveryClientFunc
			if tt.args.getDiscoveryClientFuncFactory != nil {
				getDicoveryClientFunc = tt.args.getDiscoveryClientFuncFactory(tt.args.namespace != v1.NamespaceNone)
			}
			var getDyamicClientFunc getDynamicClientFunc
			if tt.args.getDynamicClientFunc != nil {
				getDyamicClientFunc = tt.args.getDynamicClientFunc(tt.listResult...)
			}
			err = backupResource(tt.args.resourceKind, tt.args.namespace, testDir, false,
				tt.args.getConfigFunc, getDyamicClientFunc, getDicoveryClientFunc,
				tt.args.openFileFunc)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("BackupResource() error = %v, wantErr %v", err, tt.wantErr)
				} else {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				for _, expectedResource := range tt.expected {
					resourceName, _, err := unstructured.NestedString(expectedResource.Object, "metadata", "name")
					assert.NoError(t, err)
					var fileAbsolutPath string
					if tt.args.namespace == v1.NamespaceNone {
						fileAbsolutPath = fmt.Sprintf(testDir+"/%s_%s.yaml",
							resourceName, testResourceKindLowerCase)
					} else {
						fileAbsolutPath = fmt.Sprintf(testDir+"/%s_%s_%s.yaml",
							resourceName, testResourceKindLowerCase, testNamespace)
					}

					assert.FileExists(t, fileAbsolutPath)
					content, err := os.ReadFile(fileAbsolutPath)
					assert.NoError(t, err)
					var actual map[string]interface{}
					assert.NoError(t, yaml.Unmarshal(content, &actual))
					fmt.Println(fileAbsolutPath)
					assert.Equal(t, expectedResource.Object, actual)
				}
			}
		})
	}
}

func TestBackupResource_WithArchive(t *testing.T) {
	tests := []testCase{
		{
			name: "error creating zip archive file",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc:          okGetDynamicClientFuncFactory,
				namespace:                     testNamespace,
				openFileFunc: func(fileAbsolutePath string) (io.WriteCloser, error) {
					return nil, errOp
				},
			},
			wantErr: true,
			errMsg: fmt.Sprintf("error creating archive file %s_%s.zip: something happened",
				testResourceKindLowerCase, testNamespace),
		},
		{
			name: "success",
			args: args{
				resourceKind:                  testResourceKindLowerCase,
				namespace:                     testNamespace,
				getConfigFunc:                 okGetConfig,
				getDiscoveryClientFuncFactory: okGetDiscoveryFuncFactory,
				getDynamicClientFunc:          okGetDynamicClientFuncFactory,
				openFileFunc:                  defaultOpenFileFunc,
			},
			wantErr:    false,
			listResult: []runtime.Object{obj},
			expected:   []*unstructured.Unstructured{objAfterBackup},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testDir string
			var err error
			if tt.args.openFileFunc != nil && !tt.wantErr {
				testDir, err = os.MkdirTemp("", "unittest")
				t.Cleanup(func() {
					if err := os.RemoveAll(testDir); err != nil {
						t.Log(err.Error())
					}
				})
				assert.NoError(t, err)
			}
			var getDyamicClientFunc getDynamicClientFunc
			if tt.args.getDynamicClientFunc != nil {
				getDyamicClientFunc = tt.args.getDynamicClientFunc(tt.listResult...)
			}
			err = backupResource(tt.args.resourceKind, tt.args.namespace, testDir, true,
				tt.args.getConfigFunc, getDyamicClientFunc, tt.args.getDiscoveryClientFuncFactory(tt.args.namespace != v1.NamespaceNone), tt.args.openFileFunc)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("BackupResource() error = %v, wantErr %v", err, tt.wantErr)
				} else {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				fileAbsolutPath := fmt.Sprintf(testDir+"/%s_%s.zip",
					testResourceKindLowerCase, testNamespace)
				assert.FileExists(t, fileAbsolutPath)
				r, err := zip.OpenReader(fileAbsolutPath)
				assert.NoError(t, err)

				t.Cleanup(func() {
					r.Close()
				})

				for _, actualResourceFile := range r.File {
					rd, err := actualResourceFile.Open()
					assert.NoError(t, err)
					var actual map[string]interface{}
					err = yaml.NewDecoder(rd).Decode(&actual)
					assert.NoError(t, err)
					for _, expectedResource := range tt.expected {
						expectedResourceName, _, err := unstructured.NestedString(expectedResource.Object, "metadata", "name")
						assert.NoError(t, err)
						actualResourceName, _, err := unstructured.NestedString(expectedResource.Object, "metadata", "name")
						assert.NoError(t, err)
						if expectedResourceName == actualResourceName {
							assert.Equal(t, expectedResource.Object, actual)

						}
					}
				}
			}
		})
	}
}
