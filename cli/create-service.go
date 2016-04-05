package main

import (
	"log"
	"strconv"
	"time"

	"github.com/99designs/ecs-cli"
	"github.com/99designs/ecs-cli/compose"
	"github.com/99designs/ecs-cli/templates"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type CreateServiceCommandInput struct {
	ClusterName    string
	ProjectName    string
	ComposeFile    string
	HealthCheckUrl string
}

func CreateServiceCommand(ui *Ui, input CreateServiceCommandInput) {
	stack, _ := ecscli.FindServiceStack(cfnSvc, input.ClusterName, input.ProjectName)
	if stack != nil {
		ui.Fatalf("A service already exists for %q in cluster %q. Use `ecs-deploy` or `ecs-up update-service`",
			input.ProjectName, input.ClusterName)
	}

	ui.Printf("Generating task definition from %s", input.ComposeFile)
	taskDefinitionInput, err := compose.TransformComposeFile(input.ComposeFile, input.ProjectName)
	if err != nil {
		ui.Fatal(err)
	}

	ui.Printf("Registering a task for %s", input.ProjectName)
	resp, err := ecsSvc.RegisterTaskDefinition(taskDefinitionInput)
	if err != nil {
		ui.Fatal(err)
	}
	ui.Printf("Registered task definition %s:%d", *resp.TaskDefinition.Family, *resp.TaskDefinition.Revision)

	network, err := ecscli.FindNetworkStack(cfnSvc, input.ClusterName)
	if err != nil {
		ui.Fatal(err)
	}

	log.Printf("Found cloudformation stack %s for ECS cluster", network.StackName)

	params := map[string]string{
		"ECSCluster":       input.ClusterName,
		"TaskFamily":       *resp.TaskDefinition.Family,
		"TaskDefinition":   *resp.TaskDefinition.TaskDefinitionArn,
		"Subnets":          network.Subnets,
		"Vpc":              network.Vpc,
		"ECSSecurityGroup": network.SecurityGroup,
	}

	exposedPorts := ecscli.ExposedPorts(resp.TaskDefinition)

	if len(exposedPorts) != 1 {
		ui.Fatalf("Task definition without exactly 1 host mapped port are not yet supported")
	}

	// for now this is a single value
	for container, mappings := range exposedPorts {
		for _, mapping := range mappings {
			params["ContainerName"] = container
			params["ContainerPort"] = strconv.FormatInt(*mapping.ContainerPort, 10)
			params["HealthCheckUrl"] = input.HealthCheckUrl
			params["ELBPort"] = strconv.FormatInt(*mapping.HostPort, 10)
		}
	}

	timer := time.Now()
	stackName := input.ClusterName + "-ecs-service-" + time.Now().Format("20060102-150405")

	ui.Printf("Creating service cloudformation stack %s", stackName)

	err = ecscli.CreateStack(cfnSvc, stackName, templates.EcsService(), params)
	if err != nil {
		ui.Fatal(err)
	}

	err = ecscli.PollUntilCreated(cfnSvc, stackName, func(event *cloudformation.StackEvent) {
		ui.Printf("%s\n", ecscli.FormatStackEvent(event))
	})
	if err != nil {
		ui.Fatal(err)
	}

	stackOutputs, err := ecscli.StackOutputs(cfnSvc, stackName)
	if err != nil {
		ui.Fatal(err)
	}

	ui.Printf("Waiting for service to reach a steady state.")
	err = ecscli.PollUntilTaskDeployed(ecsSvc, input.ClusterName, stackOutputs["ECSService"], *resp.TaskDefinition.TaskDefinitionArn, ui.EcsEventPrinter())
	if err != nil {
		ui.Fatal(err)
	}

	log.Printf("Service created in %s", time.Now().Sub(timer).String())
	ui.Println("Service available at", stackOutputs["ECSLoadBalancer"])
}
