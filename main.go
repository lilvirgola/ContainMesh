package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"test/ContainMesh/config"
	"test/ContainMesh/docker_functions"

	"github.com/docker/docker/api/types/image"
	"github.com/moby/term"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

// CreateVirtualEnviroment creates the virtual environment in docker given a pointer to a Docker client and a pointer to the config struct
// It returns an error if the creation fails
func CreateVirtualEnviroment(cli *client.Client, config *config.Config) error {
	// Create the networks
	err := docker_functions.CreateNetworks(cli, *config.NetworkName, *config.NumNetworks)
	if err != nil {
		return fmt.Errorf("error during the creation of the networks: %v", err)
	}
	// Create the containers
	err = docker_functions.CreateContainers(cli, *config.ImageName, *config.NumContainers, *config.NetworkName, *config.NumNetworks)
	if err != nil {
		return fmt.Errorf("error during the creation of the containers: %v", err)
	}
	// Create the links if there are more than 1 network
	if *config.NumNetworks > 1 {
		err = docker_functions.CreateLinks(cli, config)
		if err != nil {
			return fmt.Errorf("error during the creation of the links: %v", err)
		}
	}

	return nil
}

// CreateConnectScript creates a bash script to connect to the containers
// It returns an error if the creation fails
func CreateConnectScript(config *config.Config) error {
	// Nome del file bash che vogliamo creare
	filename := "connect_to_host.sh"

	// Crea il file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error during the creation of the bash script: %v", err)
	}
	defer file.Close()

	// Bash commands to write in the file
	bashScript := `#!/bin/bash
if [ -z "$1" ]; then
    echo "This script is used to enter a container by its number
	Usage:
	$0 <container_number>
	"
    exit 1
fi

case $1 in
    ''|*[!0-9]*) echo "This script is used to enter a container by its number
	Usage:
	$0 <container_number>
	"
    exit 1 ;;
    *) ;;
esac

if [ "$1" -ge "` + strconv.Itoa(*config.NumContainers**config.NumNetworks) + `" ]; then
    echo "the container number must be less than ` + strconv.Itoa(*config.NumContainers**config.NumNetworks) + `"
	exit 1
elif [ "$1" -lt 0 ]; then
	echo "the container number must be greater than 0"
	exit 1
else
	sudo docker container exec -it cont_` + *config.ImageName + `$1 /bin/sh
fi
`

	// Write the commands in the file
	_, err = file.WriteString(bashScript)
	if err != nil {
		return fmt.Errorf("error during the writing of the bash script: %v", err)
	}

	// Set the perms
	err = os.Chmod(filename, 0755)
	if err != nil {
		return fmt.Errorf("error during the setting of the permissions: %v", err)
	}

	fmt.Println("Bash script successfully created:", filename)
	return nil
}

// PostCreationMenu displays a menu after the creation of the virtual environment
func PostCreationMenu(config *config.Config, client *client.Client) error {
	var choice string
	for {
		fmt.Println("\t\t MENU ")
		fmt.Println("Please select an option:")
		fmt.Println("1: Print the network adjacency matrix")
		fmt.Println("2: Stop a container")
		fmt.Println("3: Restart a container")
		fmt.Println("0: Stop all the containers and networks delete them and exit")
		fmt.Scanln(&choice)
		switch choice {
		case "1":
			if *config.NumNetworks == 1 {
				fmt.Println("The adjacency matrix is not available because there is only 1 network")
			} else {
				docker_functions.PrintMatrix(&config.NetMatrix, *config.NumNetworks)
			}
			continue
		case "2":
			var containerNumber int
			fmt.Println("The containers are numbered from 0 to", *config.NumContainers**config.NumNetworks-1)
			fmt.Print("Enter the container number: ")
			fmt.Scanln(&containerNumber)
			for containerNumber < 0 || containerNumber >= *config.NumContainers**config.NumNetworks {
				if containerNumber < 0 || containerNumber >= *config.NumContainers**config.NumNetworks {
					fmt.Println("Invalid container number")
					fmt.Scanln(&containerNumber)
				}
			}

			err := docker_functions.StopContainer(client, containerNumber, *config.ImageName)
			if err != nil {
				return fmt.Errorf("error during the stopping of the container: %v", err)
			}
			continue
		case "3":
			var containerNumber int
			fmt.Println("The containers are numbered from 0 to", *config.NumContainers**config.NumNetworks-1)
			fmt.Print("Enter the container number: ")
			fmt.Scanln(&containerNumber)
			for containerNumber < 0 || containerNumber >= *config.NumContainers**config.NumNetworks {
				if containerNumber < 0 || containerNumber >= *config.NumContainers**config.NumNetworks {
					fmt.Println("Invalid container number")
					fmt.Scanln(&containerNumber)
				}
			}
			err := docker_functions.RestartContainer(client, containerNumber, *config.ImageName)
			if err != nil {
				return fmt.Errorf("error during the restarting of the container: %v", err)
			}
			continue
		case "0":
			return nil
		default:
			fmt.Println("Invalid choice")
			continue
		}
	}
}

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
	err = docker_functions.DeleteAll(cli, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Build the Docker image
	if !*config.IgnoreBuild {
		err = docker_functions.BuildDockerImage(cli, config)
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
	err = CreateVirtualEnviroment(cli, config)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create the bash script to connect to the containers
	err = CreateConnectScript(config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Display the menu for the post creation options
	err = PostCreationMenu(config, cli)
	if err != nil {
		fmt.Println(err)
	}
	// Remove all the containers and networks
	err = docker_functions.DeleteAll(cli, config)
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
