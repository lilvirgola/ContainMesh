# ContainMesh
[![Build Status](https://travis-ci.org/joemccann/dillinger.svg?branch=master)](https://travis-ci.org/joemccann/dillinger)

> A Go program for creating and managing containers and networks based on an adjacency matrix.

# Description

ContainMesh is a tool for simulate a distributed system enviroment in docker given a docker file or a docker image. Network configurations are defined by the user via an adjacency matrix, allowing flexible modeling of network connections.
# Features

- Creates multiple containers in isolated networks.
- Configures networks based on a user-defined adjacency matrix.

# Installation

Make sure you have Go installed (Go 1.23.2 is recommended) and the Docker engine installed. Then, clone the repository and build the project:

```bash
git clone https://github.com/lilvirgola/ContainMesh
cd ContainMesh
go build
```
# Usage

To run the program, you may need `sudo` privilege (if you don't a docker rootless installation).
 ```bash
 sudo ./ContainMesh -i erlang -p -n 1 -c 1    
 ```
 this pulls the erlang image from the docker and launch a container in a network, then creates a bash script that you can use to connect to the container.
 ```bash
 ./connect_to_host.sh 0
 ```
 To see all options see the helper of the program:
 ```bash
 ./ContainMesh -h
 ```
 
# Contributing

If you'd like to contribute to this project, feel free to fork the repository and submit a pull request. You are also welcome to open issues to report bugs or suggest new features.

# License

This project is licensed under the MIT License.
