package scale

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"reflect"

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

func (p *PodAutoScaler) ScaleUp() error {
	log.Infof("Scaleup call, deployment api call : p.Client.Deployments(" + p.Namespace + ").Get(" + p.Deployment + ") ")
	deployment, err := p.Client.Deployments(p.Namespace).Get(p.Deployment)
	if err != nil {
		return errors.Wrap(err, "Failed to get deployment from kube server, no scale up occured")
	}

	currentReplicas := deployment.Spec.Replicas

	if currentReplicas >= int32(p.Max) {
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

	currentReplicas := deployment.Spec.Replicas

	if currentReplicas <= int32(p.Min) {
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

func (p *PodAutoScaler) SetReplicas(newReplicas int32, deployment *extensions.Deployment) error {
	currentReplicas := deployment.Spec.Replicas

	if newReplicas < int32(p.Min) {
		return errors.New(fmt.Sprintf("Set replicas called with value %d below minimum of %d", newReplicas, p.Min))
	}
	if newReplicas > int32(p.Max) {
		return errors.New(fmt.Sprintf("Set replicas called with value %d above maximum of %d", newReplicas, p.Max))
	}

	deployment.Spec.Replicas = newReplicas

	log.Infof("SetReplicas call, deployment api call : p.Client.Deployments(" + p.Namespace + ").Update(" + p.Deployment + ") ")
	deployment, err := p.Client.Deployments(p.Namespace).Update(deployment)
	return err

	log.Infof("Scale successful. Replicas: %d", deployment.Spec.Replicas)
	return nil
}
