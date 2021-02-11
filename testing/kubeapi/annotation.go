package kubeapi

import (
	"fmt"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckAnnotations returns true if given deployments have the annotation key=value
func (k *KubeAPI) CheckAnnotations(namespace, annotationKey, annotationValue string, deps []string) (bool, error) {
	has := false
	for _, name := range deps {
		deployment, err := k.GetDeployment(namespace, name)
		if err != nil {
			return false, err
		}
		fmt.Println(deployment.Spec.Template.ObjectMeta)
		has = meta_v1.HasAnnotation(deployment.Spec.Template.ObjectMeta, annotationKey)
		if !has {
			return false, nil
		}

		val := deployment.Spec.Template.ObjectMeta.Annotations[annotationKey]
		if val != annotationValue {
			return false, nil
		}
	}

	return has, nil
}
