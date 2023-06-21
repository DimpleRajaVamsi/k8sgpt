/*
Copyright 2023 The K8sGPT Authors.
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

package analyzer

import (
	"fmt"

	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	kcv1alpha1 "github.com/vmware-tanzu/carvel-kapp-controller/pkg/apis/kappctrl/v1alpha1"
	"k8s.io/client-go/rest"
)

type CarvelAppAnalyzer struct{}

func (CarvelAppAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "apps"
	apiDoc := kubernetes.K8sApiReference{
		Kind:          kind,
		ApiVersion:    kcv1alpha1.SchemeGroupVersion,
		OpenapiSchema: a.OpenapiSchema,
	}

	config := a.Client.GetConfig()
	config.ContentConfig.GroupVersion = &kcv1alpha1.SchemeGroupVersion
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	config.APIPath = "/apis"

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	var list kcv1alpha1.AppList
	err = restClient.Get().Resource(kind).Do(a.Context).Into(&list)
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]common.PreAnalysis{}
	for _, app := range list.Items {
		for _, condition := range app.Status.Conditions {
			if (condition.Type == "ReconcileFailed" || condition.Type == "DeleteFailed") && condition.Status == "True" {
				var failure common.Failure
				failure.Text = app.Status.UsefulErrorMessage
				failure.KubernetesDoc = apiDoc.GetApiDocV2("metadata.name")
				preAnalysis[fmt.Sprintf("%s/%s", app.Namespace, app.Name)] = common.PreAnalysis{
					CarvelApp:      app,
					FailureDetails: []common.Failure{failure},
				}
				break
			}
		}
	}

	for key, value := range preAnalysis {
		var currentAnalysis = common.Result{
			Kind:  kind,
			Name:  key,
			Error: value.FailureDetails,
		}
		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, nil
}
