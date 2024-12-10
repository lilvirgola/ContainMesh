package utils

import (
	"ContainMesh/config"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/client"
)

var choices = []string{"Print the network adjacency matrix", "Stop a container", "Restart a container", "Exit"}

type menu struct {
	cursor int
	choice string
}

func (m menu) Init() tea.Cmd {
	return nil
}

func (m menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		case "enter":
			// Send the choice on the channel and exit.
			m.choice = choices[m.cursor]
			return m, tea.Quit

		case "down", "j":
			m.cursor++
			if m.cursor >= len(choices) {
				m.cursor = 0
			}

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(choices) - 1
			}
		}
	}

	return m, nil
}

func (m menu) View() string {
	s := strings.Builder{}
	s.WriteString("\t\tMENU\nChose what do you want to do: \n\n")

	for i := 0; i < len(choices); i++ {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(choices[i])
		s.WriteString("\n")
	}
	s.WriteString("\n(press q to quit)\n\n")

	return s.String()
}

// Menu displays a menu after the creation of the virtual environment
func Menu(config *config.Config, client *client.Client) error {

	// Run returns the model as a tea.Model.
	for {
		p := tea.NewProgram(menu{})
		m, err := p.Run()
		if err != nil {
			return fmt.Errorf("error on the contruction of the menu: %v", err)
		}
		if m, ok := m.(menu); ok {

			switch m.choice {
			case choices[0]:
				if *config.NumNetworks == 1 {
					fmt.Println("The adjacency matrix is not available because there is only 1 network")
				} else {
					PrintMatrix(&config.NetMatrix, *config.NumNetworks)
				}
			case choices[1]:
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

				err := StopContainer(client, containerNumber, *config.ImageName)
				if err != nil {
					return fmt.Errorf("error during the stopping of the container: %v", err)
				}

			case choices[2]:
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
				err := RestartContainer(client, containerNumber, *config.ImageName)
				if err != nil {
					return fmt.Errorf("error during the restarting of the container: %v", err)
				}

			case choices[3]:
				fmt.Println("Exiting...")
				return nil
			default:
				fmt.Println("No choice selected, quitting...")
				return nil

			}
		} else {
			return fmt.Errorf("error during the type assertion of the menu model")
		}
		p.Quit()
	}
}

var (
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	dotStyle      = helpStyle.UnsetMargins()
	durationStyle = dotStyle
	appStyle      = lipgloss.NewStyle().Margin(1, 2, 0, 2)
)

type resultMsg struct {
	duration time.Duration
	msg      string
}

func (r resultMsg) String() string {
	if r.duration == 0 {
		return dotStyle.Render(strings.Repeat(".", 30))
	}
	return fmt.Sprintf("Docker: %s %s", r.msg,
		durationStyle.Render(r.duration.String()))
}

type loading struct {
	spinner  spinner.Model
	results  []resultMsg
	quitting bool
}

func newLoadingModel() loading {
	const numLastResults = 5
	s := spinner.New()
	s.Style = spinnerStyle
	return loading{
		spinner: s,
		results: make([]resultMsg, numLastResults),
	}
}

func (m loading) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m loading) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case resultMsg:
		m.results = append(m.results[1:], msg)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m loading) View() string {
	var s string

	if m.quitting {
		s += "That’s all for today!"
	} else {
		s += m.spinner.View() + " Setting up the environment..."
	}

	s += "\n\n"

	for _, res := range m.results {
		s += res.String() + "\n"
	}

	if m.quitting {
		s += "\n\n"

	}

	return appStyle.Render(s)
}

// LoadingSpinner creates a spinner that simulates the loading of the containers and networks
func LoadVirtualEnv(cli *client.Client, config *config.Config) error {
	p := tea.NewProgram(newLoadingModel())

	go CreateVirtualEnviroment(cli, config, p)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error during the execution of the spinner: %v", err)
	}
	return nil
}

type ending struct {
	spinner  spinner.Model
	results  []resultMsg
	quitting bool
}

func newEndingModel() ending {
	const numLastResults = 5
	s := spinner.New()
	s.Style = spinnerStyle
	return ending{
		spinner: s,
		results: make([]resultMsg, numLastResults),
	}
}

func (m ending) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m ending) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case resultMsg:
		m.results = append(m.results[1:], msg)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m ending) View() string {
	var s string

	if m.quitting {
		s += "That’s all for today!"
	} else {
		s += m.spinner.View() + " Deleting the environment..."
	}

	s += "\n\n"

	for _, res := range m.results {
		s += res.String() + "\n"
	}

	if m.quitting {
		s += "\n\n"

	}

	return appStyle.Render(s)
}

// LoadingSpinner creates a spinner that simulates the loading of the containers and networks
func DeleteVirtualEnv(cli *client.Client, config *config.Config) error {
	p := tea.NewProgram(newEndingModel())

	go DeleteAll(cli, config, p)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error during the execution of the spinner: %v", err)
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

	fmt.Printf("Bash script successfully created:%s \n\n", filename)
	return nil
}
