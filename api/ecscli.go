package api

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type NetworkOutputs struct {
	StackName     string
	Vpc           string
	Subnets       string
	SecurityGroup string
}

func FindServiceStack(svc *cloudformation.CloudFormation, clusterName, taskFamily string) (*cloudformation.Stack, error) {
	serviceStacks, err := FindStacksByOutputs(svc, map[string]string{
		"StackType":  "ecs-former::ecs-service",
		"ECSCluster": clusterName,
		"TaskFamily": taskFamily,
	})
	if len(serviceStacks) == 0 {
		return nil, fmt.Errorf(
			"Failed to find a cloudformation stack for task %q, cluster %q",
			taskFamily,
			clusterName,
		)
	}
	return serviceStacks[0], err
}

func FindNetworkStack(svc cfnInterface, clusterName string) (NetworkOutputs, error) {
	stackName := clusterName + "-network"

	outputs, err := StackOutputs(svc, stackName)
	if err != nil {
		return NetworkOutputs{StackName: stackName}, err
	}

	if err := outputs.RequireKeys("Vpc", "Subnets", "SecurityGroup"); err != nil {
		return NetworkOutputs{StackName: stackName}, err
	}

	return NetworkOutputs{
		StackName:     stackName,
		Vpc:           outputs["Vpc"],
		Subnets:       outputs["Subnets"],
		SecurityGroup: outputs["SecurityGroup"],
	}, nil
}

func FindAllStacksForCluster(svc cfnInterface, clusterName string) ([]*cloudformation.Stack, error) {
	stacks, err := FindStacksByOutputs(svc, map[string]string{
		"ECSCluster": clusterName,
	})

	networkStack, err := FindStacksByName(svc, clusterName+"-network")
	if err == nil {
		stacks = append(stacks, networkStack...)
	}

	return stacks, nil
}
