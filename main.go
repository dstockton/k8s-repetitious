package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	rest2 "k8s.io/client-go/rest"
)

// Caches the aggregated deployment lists
var aggregateDeployments map[string]*v1beta1.DeploymentList

// Caches the local deployment list
var localDeployments *v1beta1.DeploymentList

type configAndClientSet struct {
	config    *rest2.Config
	clientset *kubernetes.Clientset
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	// Initialise the empty aggregated deployment lists
	aggregateDeployments = make(map[string]*v1beta1.DeploymentList)

	// Load list of repetious endpoints
	remotes := strings.Split(getEnv("REPETIOUS_REMOTES", "127.0.0.1:3000"), ",")

	remotePollDelay, _ := time.ParseDuration(getEnv("REMOTE_POLL_DELAY", "5"))
	localPollDelay, _ := time.ParseDuration(getEnv("LOCAL_POLL_DELAY", "5"))

	// Refresh from remote repetitious API every X seconds
	go getRemoteDeploymentsLoop(remotes, remotePollDelay)

	// Refresh from local kube API every X seconds
	go getLocalDeploymentsLoop(localPollDelay)

	// Setup gin (HTTP server)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Serve the static frontend
	router.Use(static.Serve("/", static.LocalFile("./views", true)))

	// Define API routes
	api := router.Group("/api")
	{
		api.GET("/aggregate-deployments", aggregateDeploymentHandler)
		api.GET("/deployments", deploymentHandler)
	}

	// Start the listener
	fmt.Println("Listening on port 3000...")
	router.Run(":3000")
}

func getRemoteDeployments(url string) *v1beta1.DeploymentList {
	spaceClient := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println(err)
		return &v1beta1.DeploymentList{}
	}

	req.Header.Set("User-Agent", "repetitious-k8s-dashboard")

	res, getErr := spaceClient.Do(req)
	if getErr != nil {
		log.Println(getErr)
		return &v1beta1.DeploymentList{}
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Println(readErr)
		return &v1beta1.DeploymentList{}
	}

	deploymentList := v1beta1.DeploymentList{}
	jsonErr := json.Unmarshal(body, &deploymentList)
	if jsonErr != nil {
		log.Println(jsonErr)
		return &v1beta1.DeploymentList{}
	}

	return &deploymentList
}

func getRemoteDeploymentsLoop(remotes []string, delay time.Duration) {
	// Loop
	for {
		time.Sleep(delay * time.Second)
		for _, url := range remotes {
			aggregateDeployments[url] = getRemoteDeployments("http://" + url + "/api/deployments")
		}
	}
}

func getLocalDeploymentsLoop(delay time.Duration) {
	config, err := rest2.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Loop
	for {
		time.Sleep(delay * time.Second)
		localDeployments = getLocalDeployments("", clientset)
	}
}

// User home directory (win vs other)
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// Get deployment list
func getLocalDeployments(namespace string, cs *kubernetes.Clientset) *v1beta1.DeploymentList {
	deployments, err := cs.ExtensionsV1beta1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		fmt.Println(err.Error())
		return &v1beta1.DeploymentList{}
	}
	return deployments
}

// Just return the deployment struct as JSON
func aggregateDeploymentHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, aggregateDeployments)
}

// Just return the deployment struct as JSON
func deploymentHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, localDeployments)
}
