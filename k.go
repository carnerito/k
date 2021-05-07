package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	kubectlExecutable string
)

type Config struct {
	Contexts []struct {
		Context map[string]string
		Name    string
	} `yaml:"contexts"`
	CurrentContext string `yaml:"current-context"`
}

func init() {
	kubectlExecutablePath, err := exec.LookPath("kubectl")
	if err != nil {
		fmt.Println("kubectl not found")
		os.Exit(1)
	}
	kubectlExecutable = kubectlExecutablePath
}

func main() {

	// Open $HOME/.kube/conf file
	configFilePath := fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
	kubeConfFile, err := os.Open(configFilePath)
	if err != nil {
		panic(err)
	}
	defer func(kubeConfFile *os.File) {
		err := kubeConfFile.Close()
		if err != nil {
			panic(err)
		}
	}(kubeConfFile)

	// Read entire kube conf file and try unmarshal it to Config
	kubeConfData, err := ioutil.ReadAll(kubeConfFile)
	if err != nil {
		panic(err)
	}

	var kubeConf Config
	err = yaml.Unmarshal(kubeConfData, &kubeConf)
	if err != nil {
		panic(err)
	}

	// Extract contexts from kube config
	var contexts []string
	for _, c := range kubeConf.Contexts {
		contexts = append(contexts, c.Name)
	}

	// Create context prompt and offer extracted contexts as possible answers
	contextPrompt := selectPrompt("Select context", contexts, 10, false)
	_, selectedContext, err := contextPrompt.Run()
	if err != nil {
		panic(err)
	}

	// Set selected context
	err = kubeSetContext(selectedContext)
	if err != nil {
		panic(err)
	}

	// Ask if user want to set namespace too
	setNamespacePrompt := selectPrompt("Set Namespace for current context?", []string{"no", "yes"}, 2, false)
	_, setNamespace, err := setNamespacePrompt.Run()
	if err != nil {
		panic(err)
	}

	// If user want to set namespace try to set namespace for current context, else do nothing
	if setNamespace == "yes" {
		// Get namespaces for current context
		namespaces := kubeGetNamespace(selectedContext)

		// Create namespace prompt and offer fetched namespaces as possible answers
		namespacePrompt := selectPrompt("Select namespace", namespaces, 10, false)
		_, selectedNamespace, err := namespacePrompt.Run()
		if err != nil {
			panic(err)
		}

		// Set namespace for current context
		err = kubeSetNamespace(selectedNamespace)
		if err != nil {
			panic(err)
		}
	}
}

func selectPrompt(label string, items []string, size int, search bool) promptui.Select {

	if search {

		return promptui.Select{
			Label:        label,
			Items:        items,
			Size:         size,
			CursorPos:    0,
			HideHelp:     true,
			HideSelected: true,
			Searcher: func(input string, index int) bool {
				if strings.Contains(items[index], input) {
					return true
				}
				return false
			},
			StartInSearchMode: true,
		}

	} else {

		return promptui.Select{
			Label:        label,
			Items:        items,
			Size:         size,
			CursorPos:    0,
			HideHelp:     true,
			HideSelected: false,
		}

	}
}

func kubeSetContext(context string) error {

	c := exec.Cmd{
		Path:   kubectlExecutable,
		Args:   []string{kubectlExecutable, "config", "use-context", context},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	return c.Run()
}

func kubeSetNamespace(namespace string) error {

	namespaceSelector := fmt.Sprintf("--namespace=%s", namespace)

	c := exec.Cmd{
		Path:   kubectlExecutable,
		Args:   []string{kubectlExecutable, "config", "set-context", "--current", namespaceSelector},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	return c.Run()
}

func kubeGetNamespace(context string) []string {

	// Create buffer where 'get namespace command' output will be stored and try to execute command.
	// Note that this call can take some time depending on Kubernetes cluster utilisation, network conditions, etc...
	buff := new(bytes.Buffer)
	getNamespaceCommand := exec.Cmd{
		Path:   kubectlExecutable,
		Args:   []string{kubectlExecutable, "get", "namespace", "--context", context},
		Stdout: buff,
		Stderr: os.Stderr,
	}
	err := getNamespaceCommand.Run()
	if err != nil {
		panic(err)
	}

	// Placeholder for namespaces returned from getNamespacesCommand
	var namespaces []string

	// Scan buffer line by line (default scanner behavior)
	scanner := bufio.NewScanner(buff)
	firstLine := true
	for scanner.Scan() {
		if firstLine {
			// Skip first line since it is header - "NAME                  STATUS   AGE"
			firstLine = false
			continue
		}

		// Tokenize line. Line: 'default               Active   14d' will be converted to ["default", "Active", "14d"]
		lineTokenized := strings.Fields(scanner.Text())
		// Append lines first element to namespaces
		namespaces = append(namespaces, lineTokenized[0])
	}

	// Check if Scanner encountered non-EOF error
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return namespaces
}
