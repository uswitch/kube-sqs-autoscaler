package scale

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	conf "github.com/uswitch/kube-sqs-autoscaler/conf"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

type KubeClient interface {
	Deployments(namespace string) kclient.DeploymentInterface
}

type PodAutoScaler struct {
	Client            KubeClient
	Max               int
	Min               int
	Deployment        string
	Namespace         string
	ScaleUpAmount     float64
	ScaleDownAmount   float64
	ScaleUpOperator   string
	ScaleDownOperator string
}

func NewPodAutoScaler(myConf conf.MyConfType) *PodAutoScaler {
	log.Infof("Configuring with namespace " + myConf.KubernetesNamespace)
	config, err := restclient.InClusterConfig()
	if err != nil {
		panic("Failed to configure incluster config")
	}

	k8sClient, err := kclient.New(config)
	if err != nil {
		panic("Failed to configure client")
	}
	return &PodAutoScaler{
		Client:            k8sClient,
		Min:               myConf.MinPods,
		Max:               myConf.MaxPods,
		Deployment:        myConf.KubernetesDeploymentName,
		Namespace:         myConf.KubernetesNamespace,
		ScaleUpAmount:     myConf.ScaleUpAmount,
		ScaleDownAmount:   myConf.ScaleDownAmount,
		ScaleUpOperator:   myConf.ScaleUpOperator,
		ScaleDownOperator: myConf.ScaleDownOperator,
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

func (p *PodAutoScaler) Scale(direction Direction) (changed bool, err error) {
	var newReplicas int

	log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "Namespace": p.Namespace}).Infof("Scale " + string(direction) + " call")
	deployment, err := p.Client.Deployments(p.Namespace).Get(p.Deployment)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("Failed to get deployment from kube server, no scale %v occured", direction))
	}

	currentReplicas := int(deployment.Spec.Replicas)

	if direction == UP {
		switch p.ScaleUpOperator {
		case "*":
			newReplicas = int(float64(currentReplicas) * p.ScaleUpAmount)
		case "+":
			newReplicas = int(float64(currentReplicas) + p.ScaleUpAmount)
		case "-":
			newReplicas = int(float64(currentReplicas) - p.ScaleUpAmount)
		case "/":
			newReplicas = int(float64(currentReplicas) / p.ScaleUpAmount)
		}
	} else {
		switch p.ScaleDownOperator {
		case "*":
			newReplicas = int(float64(currentReplicas) * p.ScaleDownAmount)
		case "+":
			newReplicas = int(float64(currentReplicas) + p.ScaleDownAmount)
		case "-":
			newReplicas = int(float64(currentReplicas) - p.ScaleDownAmount)
		case "/":
			newReplicas = int(float64(currentReplicas) / p.ScaleDownAmount)
		}
	}
	newReplicas = max(min(newReplicas, p.Max), p.Min) // Force to permitted range
	if newReplicas == currentReplicas {
		log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "Namespace": p.Namespace, "maxPods": p.Max, "minPods": p.Min, "currentReplicas": currentReplicas}).Info("Target replicas = currentReplicas, no change needed")
		return false, nil
	}

	deployment.Spec.Replicas = int32(newReplicas)

	log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "newReplicas": newReplicas}).Infof("SetReplicas call")
	_, err = p.Client.Deployments(p.Namespace).Update(deployment)
	if err != nil {
		return false, errors.Wrap(err, "Failed to scale "+string(direction))
	}

	log.WithFields(log.Fields{"kubernetesDeploymentName": p.Deployment, "newReplicas": newReplicas}).Infof("Scale " + string(direction) + " successful")
	return true, nil
}
