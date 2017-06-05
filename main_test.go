package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"

	conf "github.com/uswitch/kube-sqs-autoscaler/conf"
	"github.com/uswitch/kube-sqs-autoscaler/scale"
	mainsqs "github.com/uswitch/kube-sqs-autoscaler/sqs"
)

var myConf = conf.MyConfType{
	PollInterval:             5 * time.Second,
	ScaleDownCoolPeriod:      10 * time.Second,
	ScaleUpCoolPeriod:        10 * time.Second,
	ScaleUpMessages:          100,
	ScaleDownMessages:        10,
	MaxPods:                  5,
	MinPods:                  1,
	AwsRegion:                "us-east-1",
	ScaleUpOperator:          "+",
	ScaleUpAmount:            1.0,
	ScaleDownOperator:        "-",
	ScaleDownAmount:          1.0,
	SqsQueueUrl:              "example.com",
	KubernetesDeploymentName: "test",
	KubernetesNamespace:      "test",
}

func TestRunReachMinReplicas(t *testing.T) {
	log.Info("Starting TestRunReachMinReplicas")
	testConf := myConf
	testConf.PollInterval = 1 * time.Second
	testConf.ScaleDownCoolPeriod = 1 * time.Second
	testConf.KubernetesDeploymentName = "TestRunReachMinReplicas"

	p := NewMockPodAutoScaler(testConf)
	s := NewMockSqsClient()

	go Run(p, s, testConf)

	Attributes := map[string]*string{"ApproximateNumberOfMessages": aws.String("5")}
	input := &sqs.SetQueueAttributesInput{
		Attributes: Attributes,
	}
	s.Client.SetQueueAttributes(input)

	time.Sleep(10 * time.Second)
	deployment, _ := p.Client.Deployments(testConf.KubernetesDeploymentName).Get("test")
	assert.Equal(t, int32(testConf.MinPods), deployment.Spec.Replicas, "Number of replicas should be the min")
	log.Info("Pass TestRunReachMinReplicas")
}

func TestRunReachMaxReplicas(t *testing.T) {
	testConf := myConf

	log.Info("Starting TestRunReachMaxReplicas")
	testConf.PollInterval = 1 * time.Second
	testConf.ScaleUpCoolPeriod = 1 * time.Second
	testConf.KubernetesDeploymentName = "TestRunReachMaxReplicas"

	p := NewMockPodAutoScaler(testConf)
	s := NewMockSqsClient()

	go Run(p, s, testConf)

	Attributes := map[string]*string{"ApproximateNumberOfMessages": aws.String("1000")}

	input := &sqs.SetQueueAttributesInput{
		Attributes: Attributes,
	}
	s.Client.SetQueueAttributes(input)

	time.Sleep(10 * time.Second)
	deployment, _ := p.Client.Deployments(testConf.KubernetesDeploymentName).Get("test")
	assert.Equal(t, int32(testConf.MaxPods), deployment.Spec.Replicas, "Number of replicas should be the max")
	log.Info("Pass TestRunReachMaxReplicas")
}

func TestRunScaleUpCoolDown(t *testing.T) {
	testConf := myConf
	log.Info("Starting TestRunScaleUpCoolDown")
	p := NewMockPodAutoScaler(testConf)
	s := NewMockSqsClient()

	//FIX here
	//	myConfForTest := myConf
	//	myConfForTest.kubernetesDeploymentName = "test-for-TestRunScaleUpCoolDown"
	go Run(p, s, myConf)

	Attributes := map[string]*string{"ApproximateNumberOfMessages": aws.String("100")}

	input := &sqs.SetQueueAttributesInput{
		Attributes: Attributes,
	}
	s.Client.SetQueueAttributes(input)

	time.Sleep(15 * time.Second)
	deployment, _ := p.Client.Deployments(testConf.KubernetesDeploymentName).Get("test")
	assert.Equal(t, int32(4), deployment.Spec.Replicas, "Number of replicas should be 4 if cool down for scaling up was obeyed")
	log.Info("Pass TestRunReachMaxReplicas")
}

func TestRunScaleDownCoolDown(t *testing.T) {
	testConf := myConf
	log.Info("Starting TestRunScaleDownCoolDown")
	p := NewMockPodAutoScaler(myConf)
	s := NewMockSqsClient()

	go Run(p, s, myConf)

	Attributes := map[string]*string{"ApproximateNumberOfMessages": aws.String("10")}

	input := &sqs.SetQueueAttributesInput{
		Attributes: Attributes,
	}
	s.Client.SetQueueAttributes(input)

	time.Sleep(15 * time.Second)
	deployment, _ := p.Client.Deployments(testConf.KubernetesDeploymentName).Get("test")
	assert.Equal(t, int32(2), deployment.Spec.Replicas, "Number of replicas should be 2 if cool down for scaling down was obeyed")
	log.Info("Pass TestRunScaleDownCoolDown")
}

type MockDeployment struct {
	client *MockKubeClient
}

type MockKubeClient struct {
	// stores the state of Deployment s if the api server did
	Deployment *extensions.Deployment
}

func (m *MockDeployment) Get(name string) (*extensions.Deployment, error) {
	return m.client.Deployment, nil
}

func (m *MockDeployment) Update(deployment *extensions.Deployment) (*extensions.Deployment, error) {
	m.client.Deployment.Spec.Replicas = deployment.Spec.Replicas
	return m.client.Deployment, nil
}

func (m *MockDeployment) List(opts api.ListOptions) (*extensions.DeploymentList, error) {
	return nil, nil
}

func (m *MockDeployment) Delete(name string, options *api.DeleteOptions) error {
	return nil
}

func (m *MockDeployment) Create(*extensions.Deployment) (*extensions.Deployment, error) {
	return nil, nil
}

func (m *MockDeployment) UpdateStatus(*extensions.Deployment) (*extensions.Deployment, error) {
	return nil, nil
}

func (m *MockDeployment) Watch(opts api.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (m *MockDeployment) Rollback(*extensions.DeploymentRollback) error {
	return nil
}

func (m *MockKubeClient) Deployments(namespace string) kclient.DeploymentInterface {
	return &MockDeployment{
		client: m,
	}
}

func NewMockKubeClient() *MockKubeClient {
	return &MockKubeClient{
		Deployment: &extensions.Deployment{
			Spec: extensions.DeploymentSpec{
				Replicas: 3,
			},
		},
	}
}

func NewMockPodAutoScaler(conf conf.MyConfType) *scale.PodAutoScaler {
	mockClient := NewMockKubeClient()

	return &scale.PodAutoScaler{
		Client:            mockClient,
		Min:               conf.MinPods,
		Max:               conf.MaxPods,
		Deployment:        conf.KubernetesDeploymentName,
		Namespace:         conf.KubernetesNamespace,
		ScaleUpAmount:     conf.ScaleUpAmount,
		ScaleDownAmount:   conf.ScaleDownAmount,
		ScaleUpOperator:   conf.ScaleUpOperator,
		ScaleDownOperator: conf.ScaleDownOperator,
	}
}

type MockSQS struct {
	QueueAttributes *sqs.GetQueueAttributesOutput
}

func (m *MockSQS) GetQueueAttributes(*sqs.GetQueueAttributesInput) (*sqs.GetQueueAttributesOutput, error) {
	return m.QueueAttributes, nil
}

func (m *MockSQS) SetQueueAttributes(input *sqs.SetQueueAttributesInput) (*sqs.SetQueueAttributesOutput, error) {
	m.QueueAttributes = &sqs.GetQueueAttributesOutput{
		Attributes: input.Attributes,
	}
	return &sqs.SetQueueAttributesOutput{}, nil
}

func NewMockSqsClient() *mainsqs.SqsClient {
	Attributes := map[string]*string{"ApproximateNumberOfMessages": aws.String("50")}

	return &mainsqs.SqsClient{
		Client: &MockSQS{
			QueueAttributes: &sqs.GetQueueAttributesOutput{
				Attributes: Attributes,
			},
		},
		QueueUrl: "example.com",
	}
}
