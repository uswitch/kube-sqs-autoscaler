package scale

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

type KubeClient interface {
	Deployments(namespace string) kclient.DeploymentInterface
}

type PodAutoScaler struct {
	Client     KubeClient
	Max        int
	Min        int
	Deployment string
	Namespace  string
}

func NewPodAutoScaler(kubernetesDeploymentName string, kubernetesNamespace string, max int, min int) *PodAutoScaler {
	log.Infof("Configuring with namespace " + kubernetesNamespace)
	config, err := restclient.InClusterConfig()
	if err != nil {
		panic("Failed to configure incluster config")
	}

	k8sClient, err := kclient.New(config)
	if err != nil {
		panic("Failed to configure client")
	}
	return &PodAutoScaler{
		Client:     k8sClient,
		Min:        min,
		Max:        max,
		Deployment: kubernetesDeploymentName,
		Namespace:  kubernetesNamespace,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Direction string

const (
	UP   Direction = "up"
	DOWN Direction = "down"
)

func (p *PodAutoScaler) Scale(direction Direction) (changed boolean, err error) {
	var newReplicas int

	log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "Namespace": p.Namespace}).Infof("Scale " + string(direction) + " call")
	deployment, err := p.Client.Deployments(p.Namespace).Get(p.Deployment)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to get deployment from kube server, no scale %v occured", direction))
	}

	currentReplicas := int(deployment.Spec.Replicas)

	if direction == UP {
		newReplicas = max(currentReplicas+1, p.Max)
	} else {
		newReplicas = min(currentReplicas-1, p.Min)
	}
	if newReplicas == currentReplicas {
		log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "Namespace": p.Namespace, "maxPods": p.Max, "minPods": p.Min, "currentReplicas": currentReplicas}).Info("No change needed")
		return nil
	}

	deployment.Spec.Replicas = int32(newReplicas)

	log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "newReplicas": newReplicas}).Infof("SetReplicas call")
	_, err := p.Client.Deployments(p.Namespace).Update(deployment)
	if err != nil {
		return errors.Wrap(err, "Failed to scale "+string(direction))
	}

	log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "newReplicas": newReplicas}).Infof("Scale " + string(direction) + " successful")
	return nil
}
