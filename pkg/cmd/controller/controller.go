package controller

import (
	"github.com/aws/aws-sdk-go/aws"

	"k8s.io/ingress/core/pkg/ingress"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

var (
	route53svc *Route53
	elbv2svc   *ELBV2
	ec2svc     *EC2
	noop       bool
)

// ALBController is our main controller
type ALBController struct {
	storeLister      ingress.StoreLister
	lastAlbIngresses albIngressesT
	clusterName      *string
}

// NewALBController returns an ALBController
func NewALBController(awsconfig *aws.Config, config *Config) *ALBController {
	ac := &ALBController{
		clusterName: aws.String(config.ClusterName),
	}

	route53svc = newRoute53(awsconfig)
	elbv2svc = newELBV2(awsconfig)
	ec2svc = newEC2(awsconfig)
	noop = config.Noop
	ac.lastAlbIngresses = assembleIngresses(ac)

	return ingress.Controller(ac).(*ALBController)
}

func (ac *ALBController) OnUpdate(ingressConfiguration ingress.Configuration) ([]byte, error) {
	OnUpdateCount.Add(float64(1))

	var albIngresses albIngressesT
	for _, ingress := range ac.storeLister.Ingress.List() {

		// Create a slice of albIngress's from current ingresses
		for _, albIngress := range newAlbIngressesFromIngress(ingress.(*extensions.Ingress), ac) {
			albIngresses = append(albIngresses, albIngress)
			go albIngress.createOrModify()
		}
	}

	ManagedIngresses.Set(float64(len(albIngresses)))

	// Delete albIngress's that no longer exist
	for _, albIngress := range ac.lastAlbIngresses {
		if albIngresses.find(albIngress) < 0 {
			go albIngress.delete()
		}
	}

	ac.lastAlbIngresses = albIngresses
	return []byte(""), nil
}