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

func (p *PodAutoScaler) Scale(direction Direction) error {
	var newReplicas int
	if direction != UP && direction != DOWN {
		return errors.New(fmt.Sprintf("Scale called with invalid direction ", direction))
	}

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

	err = p.SetReplicas(newReplicas, deployment)

	if err != nil {
		return errors.Wrap(err, "Failed to scale "+string(direction))
	}

	log.Infof("Scale " + string(direction) + " successful")
	return nil
}

func (p *PodAutoScaler) ScaleUp() error {
	log.Infof("Scaleup call, deployment api call : p.Client.Deployments(" + p.Namespace + ").Get(" + p.Deployment + ") ")
	deployment, err := p.Client.Deployments(p.Namespace).Get(p.Deployment)
	if err != nil {
		return errors.Wrap(err, "Failed to get deployment from kube server, no scale up occured")
	}

	currentReplicas := int(deployment.Spec.Replicas)

	if currentReplicas >= p.Max {
		log.WithFields(log.Fields{"maxPods": p.Max, "currentReplicas": currentReplicas}).Info("At max pods")
		return nil
	}

	err = p.SetReplicas(currentReplicas+1, deployment)

	if err != nil {
		return errors.Wrap(err, "Failed to scale up")
	}

	log.Infof("Scale up successful")
	return nil
}

func (p *PodAutoScaler) ScaleDown() error {
	log.Infof("Scaledown call, deployment api call : p.Client.Deployments(" + p.Namespace + ").Get(" + p.Deployment + ") ")
	deployment, err := p.Client.Deployments(p.Namespace).Get(p.Deployment)
	if err != nil {
		return errors.Wrap(err, "Failed to get deployment from kube server, no scale down occured")
	}

	currentReplicas := int(deployment.Spec.Replicas)

	if currentReplicas <= p.Min {
		log.WithFields(log.Fields{"minPods": p.Min, "currentReplicas": currentReplicas}).Info("At min pods")
		return nil
	}

	err = p.SetReplicas(currentReplicas-1, deployment)

	if err != nil {
		return errors.Wrap(err, "Failed to scale down")
	}

	log.Infof("Scale down successful")
	return nil
}

func (p *PodAutoScaler) SetReplicas(newReplicas int, deployment *extensions.Deployment) error {

	if newReplicas < p.Min {
		return errors.New(fmt.Sprintf("Set replicas called with value %d below minimum of %d", newReplicas, p.Min))
	}
	if newReplicas > p.Max {
		return errors.New(fmt.Sprintf("Set replicas called with value %d above maximum of %d", newReplicas, p.Max))
	}

	deployment.Spec.Replicas = int32(newReplicas)

	log.Infof("SetReplicas call, deployment api call : p.Client.Deployments(" + p.Namespace + ").Update(" + p.Deployment + ") ")
	deployment, err := p.Client.Deployments(p.Namespace).Update(deployment)
	return err

	log.Infof("Scale successful. Replicas: %d", deployment.Spec.Replicas)
	return nil
}
