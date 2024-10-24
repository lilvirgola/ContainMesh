package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/moby/term"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
)

// Function to create a container from an image in a specified network
func CreateNewContainer(image string, containerName string, networkName string, client *client.Client) (string, error) {
	resp, err := client.ContainerCreate(context.Background(), &container.Config{
		Image: image,
		Cmd:   []string{"tail", "-f", "/dev/null"}, // Command to keep the container running
	},
		&container.HostConfig{
			Privileged: true, // Necessary to run the container in privileged mode
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {NetworkID: networkName}, // Connect the container to the specified network
			},
		},
		nil,
		containerName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Container %s created successfully\n", containerName)
	return resp.ID, nil
}

// Function to remove a container given its ID
func RemoveContainer(cli *client.Client, containerID string) error {
	// ContainerRemove options allow you to force stop a container before removing
	removeOptions := container.RemoveOptions{
		Force: true,
	}
	// Remove the container
	if err := cli.ContainerRemove(context.Background(), containerID, removeOptions); err != nil {
		return err
	}

	fmt.Printf("Container %s removed successfully\n", containerID)
	return nil
}

// Function to build a Docker network
func CreateNetwork(name string, client *client.Client) (string, error) {
	// Create the network
	network, err := client.NetworkCreate(context.Background(), name, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return "", err
	}
	fmt.Printf("Network %s created successfully\n", name)

	return network.ID, nil
}

// Function to remove a network given its ID
func RemoveNetwork(cli *client.Client, networkID string) error {
	// Remove the network
	if err := cli.NetworkRemove(context.Background(), networkID); err != nil {
		return err
	}
	fmt.Printf("Network %s removed successfully\n", networkID)
	return nil
}

// function to remove all the containers and networks
func DeleteAll(cli *client.Client, imageName string, networkName string) error {
	// Get all the containers
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: true,
	})
	if err != nil {
		return err
	}
	var containerIDs []string
	// Filter those whose name contains the image_name
	for _, container := range containers {
		for _, name := range container.Names {
			if strings.Contains(name, imageName) {
				containerIDs = append(containerIDs, container.ID[:12])
			}
		}
	}
	// Remove all the selected containers
	for _, containerID := range containerIDs {
		err := RemoveContainer(cli, containerID)
		if err != nil {
			return err
		}
	}
	// Get all the networks
	networks, err := cli.NetworkList(context.Background(), network.ListOptions{})
	if err != nil {
		return err
	}
	var networkIDs []string
	// Filter those whose name contains the network_name
	for _, network := range networks {
		if strings.Contains(network.Name, networkName) {
			networkIDs = append(networkIDs, network.ID)
		}
	}
	// Remove all the selected networks
	for _, networkID := range networkIDs {
		err := RemoveNetwork(cli, networkID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Function to create all the containers of all the networks
func CreateContainers(cli *client.Client, imageName string, numContainers int, networkName string, numNetworks int) error {
	cont := 0
	//for each network
	for j := 0; j < numNetworks; j++ {
		netName := networkName + strconv.Itoa(j)
		//create the n containers
		for i := 0; i < numContainers; i++ {
			containerName := "cont_" + imageName + strconv.Itoa(cont)
			contId, err := CreateNewContainer(imageName, containerName, netName, cli)
			if err != nil {
				return fmt.Errorf("error during the creation of the container: %v", err)
			}
			err = cli.ContainerStart(context.Background(), contId, container.StartOptions{})
			if err != nil {
				return fmt.Errorf("error during the startup of the container: %v", err)
			}
			fmt.Printf("Container %s started successfully\n", containerName)
			cont++
		}
	}
	return nil
}

// Function to create all the networks
func CreateNetworks(cli *client.Client, networkName string, numNetworks int) error {
	for i := 0; i < numNetworks; i++ {
		netName := networkName + strconv.Itoa(i)
		_, err := CreateNetwork(netName, cli)
		if err != nil {
			return fmt.Errorf("error during the creation of the networks: %v", err)
		}
	}
	fmt.Println("All networks created successfully")
	return nil
}

// Function to get the context of the build
func GetContext(filePath string) io.Reader {
	ctx, _ := archive.TarWithOptions(filePath, &archive.TarOptions{})
	return ctx
}

// Function to build the Docker image
func BuildDockerImage(client *client.Client, imageName string, parentFolder string) error {
	// Define the build context
	buildContext := GetContext(parentFolder)

	// Configure the build options
	buildOptions := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",        // Name of the Dockerfile
		Tags:       []string{imageName}, // Name of the image

	}

	// Build the image
	buildResponse, err := client.ImageBuild(context.Background(), buildContext, buildOptions)
	if err != nil {
		return fmt.Errorf("error during the image building: %v", err)
	}
	defer buildResponse.Body.Close()
	// Shows the build output
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	jsonmessage.DisplayJSONMessagesStream(buildResponse.Body, os.Stderr, termFd, isTerm, nil)
	fmt.Printf("Image %s built successfully\n", imageName)
	return nil
}

// Function to connect 2 networks by adding a node of the first network to the second network
func ConnectNetworks(cli *client.Client, network1 int, network2 int, networkName string, imageName string, numContainers int, numNetworks int, numLinks int) error {
	netName2 := networkName + strconv.Itoa(network2)
	for i := 0; i < numLinks; i++ {
		//select container on the first network
		container1 := "cont_" + imageName + strconv.Itoa(network1*numContainers+i)
		//connect the container to the second network
		err := cli.NetworkConnect(context.Background(), netName2, container1, nil)
		if err != nil {
			return fmt.Errorf("error during the connection of the container to the network: %v", err)
		}
	}

	return nil
}

// Function to create the links between the networks given the adjacency matrix
func CreateLinks(cli *client.Client, imageName string, numContainers int, networkName string, numNetworks int, numLinks int) ([][]int, error) {
	// Create the matrix of links
	matrix := make([][]int, numNetworks)
	fmt.Println("Please replay at the following question for creating the adjacency matrix:")
	//read the matrix
	reader := bufio.NewReader(os.Stdin)
	readMatrix := true
	for readMatrix {
		for i := 0; i < numNetworks; i++ {
			matrix[i] = make([]int, numNetworks)
			for j := 0; j < numNetworks; j++ {
				if i == j {
					continue
				}
				fmt.Printf("Do you want a link between network %d and network %d (Y/N): ", i, j)
				text, _ := reader.ReadString('\n')
				text = strings.Replace(text, "\n", "", -1)
				if strings.ToUpper(text) == "Y" {
					matrix[i][j] = 1
				} else {
					matrix[i][j] = 0
				}
			}
		}
		// Print the adjacency matrix and ask if it is correct
		fmt.Println("The adjacency matrix is:")
		fmt.Print("  ")
		for i := 0; i < numNetworks; i++ {
			fmt.Printf("%d ", i)
		}
		fmt.Println()
		for i := 0; i < numNetworks; i++ {
			fmt.Printf("%d ", i)
			for j := 0; j < numNetworks; j++ {
				fmt.Printf("%d ", matrix[i][j])
			}
			fmt.Println()
		}
		fmt.Print("Is the adjacency matrix correct?(Y/N): ")
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\n", "", -1)
		if strings.ToUpper(text) == "Y" {
			readMatrix = false
		}
	}
	// Create the links
	for i := 0; i < numNetworks; i++ {
		for j := 0; j < numNetworks; j++ {
			if matrix[i][j] == 1 {
				// Connect the containers to the network
				err := ConnectNetworks(cli, i, j, networkName, imageName, numContainers, numNetworks, numLinks)
				if err != nil {
					return nil, fmt.Errorf("error during the linking of 2 networks: %v", err)
				}
			}
		}
	}

	return matrix, nil
}

// Function to create the virtual environment eg. the networks, the containers and the links
func CreateVirtualEnviroment(cli *client.Client, imageName string, numContainers int, networkName string, numNetworks int, numLinks int) ([][]int, error) {
	// Create the networks
	err := CreateNetworks(cli, networkName, numNetworks)
	if err != nil {
		return nil, fmt.Errorf("error during the creation of the networks: %v", err)
	}
	// Create the containers
	err = CreateContainers(cli, imageName, numContainers, networkName, numNetworks)
	if err != nil {
		return nil, fmt.Errorf("error during the creation of the containers: %v", err)
	}
	var matrix [][]int
	// Create the links if there are more than 1 network
	if numNetworks > 1 {
		matrix, err = CreateLinks(cli, imageName, numContainers, networkName, numNetworks, numLinks)
		if err != nil {
			return nil, fmt.Errorf("error during the creation of the links: %v", err)
		}
	}

	return matrix, nil
}

// Function to create the bash script to connect to the containers
func CreateConnectScript(imageName string, numContainers int) error {
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

if [ "$1" -ge "` + strconv.Itoa(numContainers) + `" ]; then
    echo "the container number must be less than ` + strconv.Itoa(numContainers) + `"
	exit 1
elif [ "$1" -lt 0 ]; then
	echo "the container number must be greater than 0"
	exit 1
else
	sudo docker container exec -it cont_` + imageName + `$1 /bin/bash
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

func main() {
	// Command line arguments
	imageName := flag.String("i", "test_name", "Image name")
	numContainers := flag.Int("c", 5, "Number of containers")
	numNetworks := flag.Int("n", 1, "Number of networks")
	networkName := flag.String("N", "test_network", "Network name")
	numLinks := flag.Int("l", 1, "Number of links")
	path := flag.String("p", "./", "Set the path to the parent folder that contains the dockerfile")
	IgnoreBuild := flag.Bool("b", false, "Ignore the build of the image")

	// Parsing the command line arguments
	flag.Parse()

	if *numContainers < 1 || *numNetworks < 1 || *numLinks < 1 {
		fmt.Println("The number of containers, networks and links must be greater than 0")
		os.Exit(1)
	}
	if *numContainers < *numLinks {
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
	err = DeleteAll(cli, *imageName, *networkName)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Build the Docker image
	if !*IgnoreBuild {
		err = BuildDockerImage(cli, *imageName, *path)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	// Create the virtual environment
	matrix, err := CreateVirtualEnviroment(cli, *imageName, *numContainers, *networkName, *numNetworks, *numLinks)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create the bash script to connect to the containers
	err = CreateConnectScript(*imageName, *numContainers)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Wait for the user to press Q to remove all the containers and networks
	var choice string
	for choice != "Q" {
		fmt.Println("Press Q to remove all the containers and networks, M to print the network adjacency matrix(Q/M): ")
		fmt.Scanln(&choice)
		choice = strings.ToUpper(choice)
		if choice == "M" {
			if *numNetworks == 1 {
				fmt.Println("The adjacency matrix is not available because there is only 1 network")
			} else {
				fmt.Println("The adjacency matrix is:")
				fmt.Print("  ")
				for i := 0; i < *numNetworks; i++ {
					fmt.Printf("%d ", i)
				}
				fmt.Println()
				for i := 0; i < *numNetworks; i++ {
					fmt.Printf("%d ", i)
					for j := 0; j < *numNetworks; j++ {
						fmt.Printf("%d ", matrix[i][j])
					}
					fmt.Println()
				}
			}
		}
	}
	// Remove all the containers and networks
	err = DeleteAll(cli, *imageName, *networkName)
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
