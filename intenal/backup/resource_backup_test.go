package backup

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestRemoveNullFileds(t *testing.T) {

	b, err := os.ReadFile("external-secrets.yaml")
	if err != nil {
		t.Fatal(err.Error())
	}

	var manifest map[string]interface{}

	if err := yaml.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err.Error())
	}

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("not ok converting spec")
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		t.Fatal("not ok converting spec")
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("not ok converting spec")
	}

	secContext, ok := templateSpec["securityContext"].(map[string]interface{})
	if !ok {
		fmt.Println("not ok - as map[string]interface{}")
	}

	/* 	secContext, ok := templateSpec["securityContext"].([]map[string]interface{})
	   	if !ok {
	   		fmt.Println("not ok - as []map[string]interface{}")
	   	} */

	vl := reflect.ValueOf(secContext)
	fmt.Println(secContext == nil)
	fmt.Println(vl.Type())
	delete(templateSpec, "securityContext")

	templateMetadata, ok := template["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("not ok converting metadata")
	}

	creationTimeStamp := templateMetadata["creationTimestamp"]
	//v2 := reflect.ValueOf(creationTimeStamp)
	fmt.Println(creationTimeStamp == nil)
	//fmt.Println(v2.IsZero())
	delete(templateMetadata, "creationTimestamp")

	/*
		 	if vl. {
				fmt.Println("is zero")
			}
	*/
	if err := yaml.NewEncoder(os.Stdout).Encode(manifest); err != nil {
		t.Fatal(err.Error())
	}
}

func TestRemoveEmptyFields(t *testing.T) {
	for _, tc := range []int{ /*1, 2, */ 3} {
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

			removeNullsAndEmptyValues(manifest)

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

			fmt.Println(resultBuffer.String())

			assert.Equal(t, string(expectedBytes), resultBuffer.String())
		})
	}

}
