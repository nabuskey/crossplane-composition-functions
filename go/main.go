package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"encoding/json"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	yaml "github.com/goccy/go-yaml"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type object struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          *runtime.RawExtension `json:"spec,omitempty"`
}

func main() {
	funcIo := v1alpha1.FunctionIO{}
	b, err := io.ReadAll(os.Stdin) // dumb
	if err != nil {
		setErrorAndExit(&funcIo, "could not read from stdin", err)
	}
	err = k8syaml.Unmarshal(b, &funcIo)
	if err != nil {
		setErrorAndExit(&funcIo, "could not unmarshal", err)
	}
	ipAddress, err := getIp()
	if err != nil {
		setErrorAndExit(&funcIo, "could not get IP address", err)
	}
	desired := make([]v1alpha1.DesiredResource, len(funcIo.Observed.Resources))
	for i := range funcIo.Observed.Resources {
		var obj object
		obsName := funcIo.Observed.Resources[i].Name
		jErr := json.Unmarshal(funcIo.Observed.Resources[i].Resource.Raw, &obj)
		if jErr != nil {
			setErrorAndExit(&funcIo, fmt.Sprintf("could not unmarshal %s", obsName), err)
		}
		obj.Labels["my-ip"] = ipAddress
		objBytes, err := json.Marshal(obj)
		if err != nil {
			setErrorAndExit(&funcIo, fmt.Sprintf("could not marshal back %s", obsName), err)
		}
		desired[i] = v1alpha1.DesiredResource{
			Name: obsName,
			Resource: runtime.RawExtension{
				Raw: objBytes,
			},
			ConnectionDetails: nil,
			ReadinessChecks:   nil,
		}
	}
	funcIo.Desired.Resources = desired
	printAsYaml(&funcIo)

}

func getIp() (string, error) {
	res, err := http.Get("https://checkip.amazonaws.com")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(string(body), "\n"), nil
}

func setErrorAndExit(funcIo *v1alpha1.FunctionIO, msg string, err error) {
	setError(funcIo, msg, err)
	printAsYaml(funcIo)
	os.Exit(1)
}

func setError(funcIo *v1alpha1.FunctionIO, msg string, err error) {
	r := v1alpha1.Result{
		Severity: v1alpha1.SeverityFatal,
		Message:  fmt.Sprintf("%s : %s", msg, err),
	}
	funcIo.Results = append(funcIo.Results, r)
}

func printAsYaml(funcIo *v1alpha1.FunctionIO) {
	enc := yaml.NewEncoder(os.Stdout, yaml.UseJSONMarshaler())
	err := enc.Encode(funcIo)
	if err != nil {
		panic(fmt.Sprintf("could not write to stdout: %s", err))
	}
}
