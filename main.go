package main

import (
	"ContainMesh/config"
	"ContainMesh/utils"
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/image"
	"github.com/moby/term"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

func main() {
	// Command line arguments
	config, err := config.ProcessCommandLineArgs()
	if err != nil {
		fmt.Printf("error during the parsing of the command line args: %v \n", err)
		return
	}

	if *config.NumContainers < 1 || *config.NumNetworks < 1 || *config.NumLinks < 1 {
		fmt.Println("The number of containers, networks and links must be greater than 0")
		os.Exit(1)
	}
	if *config.NumContainers < *config.NumLinks {
		fmt.Println("The number of containers must be greater than the number of links")
		os.Exit(1)
	}
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cli.Close()

	// Remove all the containers and networks if they already exist
	err = utils.DeleteVirtualEnv(cli, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Build the Docker image
	if !*config.IgnoreBuild {
		err = utils.BuildDockerImage(cli, config)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	if *config.PullImage {
		out, err := cli.ImagePull(context.Background(), "docker.io/library/"+*config.ImageName, image.PullOptions{})
		if err != nil {
			fmt.Println(err)
			return
		}
		// Shows the pull output
		termFd, isTerm := term.GetFdInfo(os.Stderr)
		jsonmessage.DisplayJSONMessagesStream(out, os.Stderr, termFd, isTerm, nil)
	}
	// Create the virtual environment
	err = utils.LoadVirtualEnv(cli, config)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create the bash script to connect to the containers
	err = utils.CreateConnectScript(config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Display the menu for the post creation options
	err = utils.Menu(config, cli)
	if err != nil {
		fmt.Println(err)
	}
	// Remove all the containers and networks
	err = utils.DeleteVirtualEnv(cli, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Remove the bash script
	err = os.Remove("connect_to_host.sh")
	if err != nil {
		fmt.Println(err)
		return
	}
}
