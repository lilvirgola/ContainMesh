package utils

import (
	"ContainMesh/config"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/gin-gonic/gin"
	"github.com/moby/term"
)

var stoppedContainers []int // List of stopped containers

// CreateNewContainer creates a new container given the image name, the container name, the network name and a pointer to a Docker client
// It returns the container ID and an error if the container creation fails
func CreateNewContainer(image string, containerName string, networkName string, client *client.Client, p *tea.Program) (string, error) {
	start := time.Now()
	resp, err := client.ContainerCreate(context.Background(), &container.Config{
		Image: image,
		Cmd:   []string{"tail", "-f", "/dev/null"}, // Keep the container running
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
	end := time.Now()
	p.Send(resultMsg{end.Sub(start), fmt.Sprintf("Container %s created successfully", containerName)})
	return resp.ID, nil
}

// RemoveContainer removes a container given its ID and a pointer to a Docker client
// It returns an error if the container removal fails
func RemoveContainer(cli *client.Client, containerID string, p *tea.Program) error {
	// ContainerRemove options allow you to force stop a container before removing
	start := time.Now()
	removeOptions := container.RemoveOptions{
		Force: true,
	}
	// Remove the container
	if err := cli.ContainerRemove(context.Background(), containerID, removeOptions); err != nil {
		return err
	}
	end := time.Now()
	p.Send(resultMsg{end.Sub(start), fmt.Sprintf("Container %s removed successfully", containerID)})
	return nil
}

// CreateNetwork creates a new network given the network name and a pointer to a Docker client
// It returns the network Docker ID and an error if the network creation fails
func CreateNetwork(name string, client *client.Client, p *tea.Program) (string, error) {
	// Create the network
	start := time.Now()
	network, err := client.NetworkCreate(context.Background(), name, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return "", err
	}
	end := time.Now()
	p.Send(resultMsg{end.Sub(start), fmt.Sprintf("Network %s created successfully", name)})

	return network.ID, nil
}

// RemoveNetwork removes a network given its ID and a pointer to a Docker client
// It returns an error if the network removal fails
func RemoveNetwork(cli *client.Client, networkID string, p *tea.Program) error {
	// Remove the network
	start := time.Now()
	if err := cli.NetworkRemove(context.Background(), networkID); err != nil {
		return err
	}
	end := time.Now()
	p.Send(resultMsg{end.Sub(start), fmt.Sprintf("Network %s removed successfully", networkID)})
	return nil
}

// DeleteAll removes all the containers and networks that contain the image_name and network_name
// It returns an error if the removal fails
func DeleteAll(cli *client.Client, config *config.Config, p *tea.Program) error {
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
			if strings.Contains(name, *config.ImageName) {
				containerIDs = append(containerIDs, container.ID[:12])
			}
		}
	}
	// Remove all the selected containers
	for _, containerID := range containerIDs {
		err := RemoveContainer(cli, containerID, p)
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
		if strings.Contains(network.Name, *config.NetworkName) {
			networkIDs = append(networkIDs, network.ID)
		}
	}
	// Remove all the selected networks
	for _, networkID := range networkIDs {
		err := RemoveNetwork(cli, networkID, p)
		if err != nil {
			return err
		}
	}
	p.Quit()
	return nil
}

// ContainerNameFromNodeNumber returns the container name given the node number and the image name
func ContainerNameFromNodeNumber(nodeNumber int, imageName string) string {
	return "cont_" + imageName + strconv.Itoa(nodeNumber)
}

// CreateContainers creates n containers given the image name, the number of containers, the network name and the number of networks and a pointer to a Docker client
// It returns an error if the container creation fails
func CreateContainers(cli *client.Client, imageName string, numContainers int, networkName string, numNetworks int, p *tea.Program) error {
	cont := 0
	//for each network
	for j := 0; j < numNetworks; j++ {
		netName := networkName + strconv.Itoa(j)
		//create the n containers
		for i := 0; i < numContainers; i++ {
			start := time.Now()
			containerName := ContainerNameFromNodeNumber(cont, imageName)
			contId, err := CreateNewContainer(imageName, containerName, netName, cli, p)
			if err != nil {
				return fmt.Errorf("error during the creation of the container: %v", err)
			}
			err = cli.ContainerStart(context.Background(), contId, container.StartOptions{})
			if err != nil {
				return fmt.Errorf("error during the startup of the container: %v", err)
			}
			end := time.Now()
			p.Send(resultMsg{end.Sub(start), fmt.Sprintf("Container %s started successfully", containerName)})
			cont++
		}
	}
	return nil
}

// StopContainer stops a container given its ID and a pointer to a Docker client
// It returns an error if the container stopping fails
func StopContainer(cli *client.Client, nodeNumber int, imageName string) error {
	containerName := ContainerNameFromNodeNumber(nodeNumber, imageName)
	containerID, err := GetContainerID(cli, containerName)
	if err != nil {
		return fmt.Errorf("error during the retrieval of the container ID: %v", err)
	}
	err = cli.ContainerStop(context.Background(), containerID, container.StopOptions{})
	// Stop the container
	if err != nil {
		return fmt.Errorf("error during the halting of the container %s:%v", containerID, err)
	}
	stoppedContainers = append(stoppedContainers, nodeNumber)
	fmt.Printf("Container %s stopped successfully\n", containerID)
	return nil
}

// RestartContainer restarts a container given its ID and a pointer to a Docker client
// It returns an error if the container restarting fails
func RestartContainer(cli *client.Client, nodeNumber int, imageName string) error {
	containerName := ContainerNameFromNodeNumber(nodeNumber, imageName)
	containerID, err := GetContainerID(cli, containerName)
	if err != nil {
		return fmt.Errorf("error during the retrieval of the container ID: %v", err)
	}
	sort.Ints(stoppedContainers)
	i := sort.SearchInts(stoppedContainers, nodeNumber)
	if stoppedContainers != nil && stoppedContainers[i] == nodeNumber {
		fmt.Printf("Container %d is stopped\n", stoppedContainers[i])
		err := cli.ContainerStart(context.Background(), containerID, container.StartOptions{})
		// Restart the container
		if err != nil {
			return fmt.Errorf("error during the restart of the container %s:%v", containerID, err)
		}
		fmt.Printf("Container %d restarted successfully\n", nodeNumber)
	} else {
		fmt.Printf("Container %d is not stopped\n", nodeNumber)
	}
	return nil
}

// GetContainerID returns the ID of a container given its name and a pointer to a Docker client
func GetContainerID(cli *client.Client, containerName string) (string, error) {
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: true,
	})
	if err != nil {
		return "", err
	}
	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				return container.ID, nil
			}
		}
	}
	return "", fmt.Errorf("container %s not found", containerName)
}

func GetStoppedContainers() []int {
	return stoppedContainers
}

// CreateNetworks creates n networks given the network name and the number of networks and a pointer to a Docker client
// It returns an error if the network creation fails
func CreateNetworks(cli *client.Client, networkName string, numNetworks int, p *tea.Program) error {
	for i := 0; i < numNetworks; i++ {
		netName := networkName + strconv.Itoa(i)
		_, err := CreateNetwork(netName, cli, p)
		if err != nil {
			return fmt.Errorf("error during the creation of the networks: %v", err)
		}
	}
	fmt.Println("All networks created successfully")
	return nil
}

// GetContext returns the context of the build given the file path
// It returns the context of the build as an io.Reader
func GetContext(filePath string) io.Reader {
	ctx, _ := archive.TarWithOptions(filePath, &archive.TarOptions{})
	return ctx
}

// BuildDockerImage builds a Docker image given a pointer to a Docker client and a pointer to the config struct
// It returns an error if the image building fails
func BuildDockerImage(client *client.Client, config *config.Config) error {
	// Define the build context
	buildContext := GetContext(*config.DockerFilePath)

	// Configure the build options
	buildOptions := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",                // Name of the Dockerfile
		Tags:       []string{*config.ImageName}, // Name of the image

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
	fmt.Printf("Image %s built successfully\n", *config.ImageName)
	return nil
}

// ConnectNetworks connects the containers of the first network to the second network given the network IDs, the container name, the image name, the number of containers, the number of networks, the number of links adn a pointer to a Docker client
// It returns an error if the connection fails
func ConnectNetworks(cli *client.Client, network1 int, network2 int, networkName string, imageName string, numContainers int, numNetworks int, numLinks int) error {
	netName2 := networkName + strconv.Itoa(network2)
	for i := 0; i < numLinks; i++ {
		//select container on the first network
		container1 := "cont_" + imageName + strconv.Itoa(network1*numContainers+i)
		//connect the container to the second network// Function to connect 2 networks by adding a node of the first network to the second network
		err := cli.NetworkConnect(context.Background(), netName2, container1, nil)
		if err != nil {
			return fmt.Errorf("error during the connection of the container to the network: %v", err)
		}
	}

	return nil
}

// CreateLinks creates the links between the networks given the pointer to a Docker client and a pointer to the config struct
// It returns an error if the linking fails
func CreateLinks(cli *client.Client, config *config.Config, p *tea.Program) error {
	if config.NetMatrix == nil {
		config.NetMatrix = *CreateMatrix(*config.NumNetworks)
	}
	// Create the links
	for i := 0; i < *config.NumNetworks; i++ {
		for j := 0; j < *config.NumNetworks; j++ {
			if (config.NetMatrix)[i][j] && i != j { // If there is a link between the networks and they are different
				start := time.Now()
				// Connect the containers to the network
				err := ConnectNetworks(cli, i, j, *config.NetworkName, *config.ImageName, *config.NumContainers, *config.NumNetworks, *config.NumLinks)
				if err != nil {
					return fmt.Errorf("error during the linking of 2 networks: %v", err)
				}
				end := time.Now()
				p.Send(resultMsg{end.Sub(start), fmt.Sprintf("Network %d linked to network %d", i, j)})
			}
		}
	}

	return nil
}

// CreateMatrix creates the adjacency matrix given the number of networks
// It returns a pointer to the adjacency matrix
func CreateMatrix(numNetworks int) *[][]bool {
	// Create the matrix of links
	matrix := make([][]bool, numNetworks)
	fmt.Println("Please replay at the following questions for creating the adjacency matrix:")
	//read the matrix
	reader := bufio.NewReader(os.Stdin)
	readMatrix := true
	for readMatrix {
		for i := 0; i < numNetworks; i++ {
			matrix[i] = make([]bool, numNetworks)
			for j := 0; j < numNetworks; j++ {
				if i == j {
					continue
				}
				fmt.Printf("Do you want a link between network %d and network %d (Y/N): ", i, j)
				text, _ := reader.ReadString('\n')
				text = strings.Replace(text, "\n", "", -1)
				if strings.ToUpper(text) == "Y" {
					matrix[i][j] = true
				} else {
					matrix[i][j] = false
				}
			}
		}
		// Print the matrix
		PrintMatrix(&matrix, numNetworks)
		fmt.Print("Is the adjacency matrix correct?(Y/N): ")
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\n", "", -1)
		if strings.ToUpper(text) == "Y" {
			readMatrix = false
		}
	}
	return &matrix
}

// PrintMatrix prints the adjacency matrix given a pointer to the matrix and the number of networks
func PrintMatrix(matrix *[][]bool, numNetwork int) {
	fmt.Println("The adjacency matrix is:")
	fmt.Print("  ")
	for i := 0; i < numNetwork; i++ {
		fmt.Printf("%d ", i)
	}
	fmt.Println()
	for i := 0; i < numNetwork; i++ {
		fmt.Printf("%d ", i)
		for j := 0; j < numNetwork; j++ {
			if (*matrix)[i][j] {
				fmt.Printf("%d ", 1)
			} else {
				if i == j {
					fmt.Print("X ")
				} else {
					fmt.Printf("%d ", 0)
				}
			}
		}
		fmt.Println()
	}
}

// CreateVirtualEnviroment creates the virtual environment given a pointer to a Docker client and a pointer to the config struct
// It returns an error if the creation fails
func CreateVirtualEnviroment(cli *client.Client, config *config.Config, p *tea.Program) error {
	// Create the networks
	err := CreateNetworks(cli, *config.NetworkName, *config.NumNetworks, p)
	if err != nil {
		return fmt.Errorf("error during the creation of the networks: %v", err)
	}
	// Create the containers
	err = CreateContainers(cli, *config.ImageName, *config.NumContainers, *config.NetworkName, *config.NumNetworks, p)
	if err != nil {
		return fmt.Errorf("error during the creation of the containers: %v", err)
	}
	// Create the links if there are more than 1 network
	if *config.NumNetworks > 1 {
		err = CreateLinks(cli, config, p)
		if err != nil {
			return fmt.Errorf("error during the creation of the links: %v", err)
		}
	}
	p.Quit()

	return nil
}

func GetGraphEncoding(config *config.Config) gin.H {
	graph := gin.H{
		"NumNetworks":       *config.NumNetworks,
		"NumContainers":     *config.NumContainers,
		"NumLinks":          *config.NumLinks,
		"StoppedContainers": stoppedContainers,
		"NetMatrix":         config.NetMatrix,
	}
	return graph
}
