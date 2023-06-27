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
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type ClusterApiAnalyzer struct{}

func (ClusterApiAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Clusters"
	apiDoc := kubernetes.K8sApiReference{
		Kind:          kind,
		ApiVersion:    clusterv1.GroupVersion,
		OpenapiSchema: a.OpenapiSchema,
	}

	config := a.Client.GetConfig()
	config.ContentConfig.GroupVersion = &clusterv1.GroupVersion
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	config.APIPath = "/apis"

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	var list clusterv1.ClusterList
	err = restClient.Get().Resource(kind).Do(a.Context).Into(&list)
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]common.PreAnalysis{}
	for _, cluster := range list.Items {
		var failures []common.Failure
		clusterInFailedState := false
		status := cluster.Status

		// Check if the Cluster Object has failure message
		if status.FailureMessage != nil {
			failures = append(failures, common.Failure{
				Text:          *status.FailureMessage,
				KubernetesDoc: apiDoc.GetApiDocV2("metadata.name"),
			})
		}

		for _, condition := range cluster.Status.Conditions {
			if condition.Severity == clusterv1.ConditionSeverityError {
				// failures = append(failures, common.Failure{
				// 	Text: condition.Reason,
				// })
				failures = append(failures, common.Failure{
					Text: condition.Message,
				})
				clusterInFailedState = true
			}
		}

		if clusterInFailedState {
			var machines clusterv1.MachineList
			err = restClient.Get().Resource("Machines").Namespace(cluster.GetObjectMeta().GetNamespace()).Do(a.Context).Into(&machines)
			if err != nil {
				return nil, err
			}
			for _, machine := range machines.Items {
				for _, condition := range machine.Status.Conditions {
					if condition.Severity == clusterv1.ConditionSeverityError {
						// failures = append(failures, common.Failure{
						// 	Text: condition.Reason,
						// })
						failures = append(failures, common.Failure{
							Text: condition.Message,
						})
					}
				}
			}
		}

		if len(failures) != 0 {
			preAnalysis[fmt.Sprintf("%s/%s", cluster.Namespace, cluster.Name)] = common.PreAnalysis{
				CapiCluster:    cluster,
				FailureDetails: failures,
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
