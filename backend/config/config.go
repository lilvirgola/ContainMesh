package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type YamlConfig struct {
	ImageSettings struct {
		DockerFilePath string `yaml:"DockerFilePath,omitempty"`
		ImageName      string `yaml:"ImageName,omitempty"`
		IgnoreBuild    bool   `yaml:"IgnoreBuild,omitempty"`
		PullImage      bool   `yaml:"PullImage,omitempty"`
	} `yaml:"ImageSettings"`
	NetworkSettings struct {
		NetworkName   string   `yaml:"NetworkName,omitempty"`
		NumLinks      int      `yaml:"NumLinks,omitempty"`
		NumContainers int      `yaml:"NumContainers,omitempty"`
		NumNetworks   int      `yaml:"NumNetworks,omitempty"`
		NetMatrix     [][]bool `yaml:"NetMatrix,omitempty"`
	} `yaml:"NetworkSettings"`
}

type Config struct {
	NumContainers  *int
	NumNetworks    *int
	NumLinks       *int
	IgnoreBuild    *bool
	PullImage      *bool
	DockerFilePath *string
	NetworkName    *string
	ImageName      *string
	YamlFilePath   *string
	NetMatrix      [][]bool
}

// ParseYamlConfig reads the yaml file and sets the values of the config struct
// It take the config struct as an argument
// It returns an error if the yaml file is not found, if the unmarshal fails, if the number of networks is not equal to the number of rows in the matrix, if the matrix is not square
func ParseYamlConfig(config *Config) error {
	filename, _ := filepath.Abs(*config.YamlFilePath)
	yamlFile, err := os.ReadFile(filename)

	if err != nil {
		return fmt.Errorf("error reading the yaml file: %v", err)
	}
	var yamlConf YamlConfig

	err = yaml.Unmarshal(yamlFile, &yamlConf)
	if err != nil {
		return fmt.Errorf("error during the unmarshal of the yaml file: %v", err)
	}
	if yamlConf.NetworkSettings.NetMatrix != nil {
		if len(yamlConf.NetworkSettings.NetMatrix) != yamlConf.NetworkSettings.NumNetworks {
			return fmt.Errorf("the number of networks is not equal to the number of rows in the matrix")
		}
		// check for the correctness of the matrix
		if len(yamlConf.NetworkSettings.NetMatrix) != len(yamlConf.NetworkSettings.NetMatrix[0]) {
			return fmt.Errorf("the matrix is not square")
		}
		config.NetMatrix = yamlConf.NetworkSettings.NetMatrix
	}
	// Set the values of the config struct
	if yamlConf.ImageSettings.ImageName != "" {
		config.ImageName = &yamlConf.ImageSettings.ImageName
	}
	if yamlConf.NetworkSettings.NumContainers != 0 {
		config.NumContainers = &yamlConf.NetworkSettings.NumContainers
	}
	if yamlConf.NetworkSettings.NumNetworks != 0 {
		config.NumNetworks = &yamlConf.NetworkSettings.NumNetworks
	}
	if yamlConf.NetworkSettings.NumLinks != 0 {
		config.NumLinks = &yamlConf.NetworkSettings.NumLinks
	}
	if yamlConf.NetworkSettings.NetworkName != "" {
		config.NetworkName = &yamlConf.NetworkSettings.NetworkName
	}
	if yamlConf.ImageSettings.DockerFilePath != "" {
		config.DockerFilePath = &yamlConf.ImageSettings.DockerFilePath
	}
	config.IgnoreBuild = &yamlConf.ImageSettings.IgnoreBuild
	config.PullImage = &yamlConf.ImageSettings.PullImage

	return nil
}

// ProcessCommandLineArgs processes the command line arguments and returns the config struct
// It returns an error if the yaml file is not found, if the unmarshal fails, if the number of networks is not equal to the number of rows in the matrix, if the matrix is not square
func ProcessCommandLineArgs() (*Config, error) {
	config := &Config{
		ImageName:      flag.String("i", "test_name", "Image name"),
		NumContainers:  flag.Int("c", 5, "Number of containers"),
		NumNetworks:    flag.Int("n", 1, "Number of networks"),
		NetworkName:    flag.String("N", "test_network", "Network name"),
		NumLinks:       flag.Int("l", 1, "Number of links"),
		DockerFilePath: flag.String("path", "./", "Set the path to the parent folder that contains the dockerfile"),
		IgnoreBuild:    flag.Bool("b", true, "Ignore the build of the image"),
		PullImage:      flag.Bool("p", false, "Pull the image from the Docker Hub"),
		YamlFilePath:   flag.String("y", "", "Yaml configuration file name"),
	}
	flag.Parse()
	if config.YamlFilePath != nil && *config.YamlFilePath != "" {
		err := ParseYamlConfig(config)
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}
