package backup

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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

			// fmt.Println(resultBuffer.String())

			assert.Equal(t, string(expectedBytes), resultBuffer.String())
		})
	}

}
