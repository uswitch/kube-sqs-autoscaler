package main

import (
	"encoding/json"
	"flag"
	io "io/ioutil"
	"time"

	log "github.com/Sirupsen/logrus"
	"os"

	"github.com/uswitch/kube-sqs-autoscaler/scale"
	"github.com/uswitch/kube-sqs-autoscaler/sqs"
)

var (
	pollInterval        time.Duration
	scaleDownCoolPeriod time.Duration
	scaleUpCoolPeriod   time.Duration
	scaleUpMessages     int
	scaleDownMessages   int
	maxPods             int
	minPods             int
	awsRegion           string

	sqsQueueUrl              string
	kubernetesDeploymentName string
	kubernetesNamespace      string
	configFile               string
)

func Run(p *scale.PodAutoScaler, sqs *sqs.SqsClient) {
	var changed bool
	lastScaleUpTime := time.Now()
	lastScaleDownTime := time.Now()

	for {
		log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName}).Info("inside polling loop")
		select {
		case <-time.After(pollInterval):
			{
				numMessages, err := sqs.NumMessages()
				if err != nil {
					log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName, "sqs-queue": sqsQueueUrl, "error": err}).Errorf("Failed to get SQS messages")
					continue
				}

				if numMessages >= scaleUpMessages {
					if lastScaleUpTime.Add(scaleUpCoolPeriod).After(time.Now()) {
						log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName}).Info("Waiting for cool off, skipping scale up ")
						continue
					}

					log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName, "scaleUpMessages": scaleUpMessages, "numMessages": numMessages}).Info("Scale up may be appropriate")
					if changed, err = p.Scale(scale.UP); err != nil {
						log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName}).Errorf("Failed scaling up: %v", err)
						continue
					}
					if changed {
						lastScaleUpTime = time.Now()
					}
				}

				if numMessages <= scaleDownMessages {
					if lastScaleDownTime.Add(scaleDownCoolPeriod).After(time.Now()) {
						log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName}).Info("Waiting for cool off, skipping scale down")
						continue
					}
					log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName, "scaleDownMessages": scaleDownMessages, "numMessages": numMessages}).Info("Scale down may be appropriate")
					if changed, err = p.Scale(scale.DOWN); err != nil {
						log.WithFields(log.Fields{"kubernetesDeploymentName": kubernetesDeploymentName}).Errorf("Failed scaling down: %v", err)
						continue
					}
					if changed {
						lastScaleDownTime = time.Now()
					}
				}
			}
		}
	}

}

type Configuration struct {
	PollInterval             time.Duration `json:"pollperiod"`
	ScaleDownCoolPeriod      time.Duration `json:"scaledowncooloff"`
	ScaleUpCoolPeriod        time.Duration `json:"scaleupcooloff"`
	ScaleDownMessages        int           `json:"scaleDownMessages"`
	ScaleUpMessages          int           `json:"scaleupmessages"`
	MaxPods                  int           `json:"maxpods"`
	MinPods                  int           `json:"minpods`
	AwsRegion                string        `json:"awsregion"`
	SqsQueueUrl              string        `json:"sqs-queue-url"`
	KubernetesDeploymentName string        `json:"kubernetesdeployment"`
	KubernetesNamespace      string        `json:"kubernetesnamespace"`
	Active                   bool          `json:"active"`
}

type ConfigurationOld struct {
	Id      int
	Key     int `json:"keyhere"`
	MaxPods int `json:"maxpods"`
}

// Returns array of configurations to use.
//TODO fix duration parsing
func parseConfigFile(configFileName string) (configList []Configuration, err error) {
	//configList = make([]Configuration, 0)
	data, err := io.ReadFile(configFileName)
	log.Info("data read as %v", string(data))
	if err != nil {
		log.Errorf("Unable to open file %v", configFileName)
		return nil, err
	}
	err = json.Unmarshal(data, &configList)
	for i := 0; i < len(configList); i += 1 {
		v := configList[i]
		log.Infof("conf %v read as %+v", i, v)
	}

	return configList, nil
}

func main() {
	flag.DurationVar(&pollInterval, "poll-period", 5*time.Second, "The interval in seconds for checking if scaling is required")
	flag.DurationVar(&scaleDownCoolPeriod, "scale-down-cool-off", 30*time.Second, "The cool off period for scaling down")
	flag.DurationVar(&scaleUpCoolPeriod, "scale-up-cool-off", 10*time.Second, "The cool off period for scaling up")
	flag.IntVar(&scaleUpMessages, "scale-up-messages", 100, "Number of sqs messages queued up required for scaling up")
	flag.IntVar(&scaleDownMessages, "scale-down-messages", 10, "Number of messages required to scale down")
	flag.IntVar(&maxPods, "max-pods", 5, "Max pods that kube-sqs-autoscaler can scale")
	flag.IntVar(&minPods, "min-pods", 1, "Min pods that kube-sqs-autoscaler can scale")
	flag.StringVar(&awsRegion, "aws-region", "", "Your AWS region")

	flag.StringVar(&sqsQueueUrl, "sqs-queue-url", "", "The sqs queue url")
	flag.StringVar(&kubernetesDeploymentName, "kubernetes-deployment", "", "Kubernetes Deployment to scale. This field is required")
	flag.StringVar(&kubernetesNamespace, "kubernetes-namespace", "default", "The namespace your deployment is running in")

	flag.StringVar(&configFile, "config-file", "", "Configuration by json file, array of maps of the above attributes")
	flag.Parse()

	if configFile != "" {
		configList, err := parseConfigFile(configFile)
		if err != nil {
			log.Errorf("Unable to parse file %v", configFile)
			return
		}
		log.Infof("config read, got %v items", len(configList))
		return
	}
	log.Info("Starting kube-sqs-autoscaler for deployment " + kubernetesDeploymentName + " and namespace " + kubernetesNamespace)
	p := scale.NewPodAutoScaler(kubernetesDeploymentName, kubernetesNamespace, maxPods, minPods)
	sqs := sqs.NewSqsClient(sqsQueueUrl, awsRegion)

	Run(p, sqs)
}
