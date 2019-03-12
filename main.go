package main

import (
	"encoding/json"
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

// Main entry point
func main() {
	// Initialise the empty aggregated deployment lists
	aggregateDeployments = make(map[string]*v1beta1.DeploymentList)

	// Load list of repetious endpoints
	httpPort := getEnv("HTTP_PORT", "8080")
	remotes := strings.Split(getEnv("REPETIOUS_REMOTES", "127.0.0.1:"+httpPort), ",")

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
	log.Println("Listening on port " + httpPort + "...")
	router.Run(":" + httpPort)
	log.Println("Exiting")
}

// Helper function to get environment variables
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Get a list of deployments from a single remote URL
func getRemoteDeployments(url string) *v1beta1.DeploymentList {
	// Setup HTTP client
	httpClient := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}

	// Initialise the request parameters...
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		// Log any error but continue gracefully
		log.Println(err)
		return &v1beta1.DeploymentList{}
	}

	// Let's be a good citizen
	req.Header.Set("User-Agent", "repetitious-k8s-dashboard-v1")

	// Make the HTTP request
	res, getErr := httpClient.Do(req)
	if getErr != nil {
		// Log any error but continue gracefully
		log.Println(getErr)
		return &v1beta1.DeploymentList{}
	}

	// Read the entire body of the HTTP response buffer
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		// Log any error but continue gracefully
		log.Println(readErr)
		return &v1beta1.DeploymentList{}
	}

	// Ready a variable for the response
	deploymentList := v1beta1.DeploymentList{}
	// Unmarshal the JSON to DeploymentList object
	jsonErr := json.Unmarshal(body, &deploymentList)
	if jsonErr != nil {
		// Log any error but continue gracefully
		log.Println(jsonErr)
		return &v1beta1.DeploymentList{}
	}

	return &deploymentList
}

// Continuously retrieve deployments from each remote endpoint
func getRemoteDeploymentsLoop(remotes []string, delay time.Duration) {
	for {
		time.Sleep(delay * time.Second)

		// For each remote agent
		for _, url := range remotes {
			aggregateDeployments[url] = getRemoteDeployments("http://" + url + "/api/deployments")
		}
	}
}

// Continuously retrieve deployments from local cluster kube API
func getLocalDeploymentsLoop(delay time.Duration) {
	// Get current cluster kube API configuration using standard conventions
	config, err := rest2.InClusterConfig()
	if err != nil {
		// It's OK to panic here as it's only at start-up
		panic(err.Error())
	}

	// Setup kube API client using aquired config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		// It's OK to panic here as it's only at start-up
		panic(err.Error())
	}

	// Loop forever...
	for {
		time.Sleep(delay * time.Second)
		localDeployments = getLocalDeployments("", clientset)
	}
}

// Get the local kube API deployment list
func getLocalDeployments(namespace string, cs *kubernetes.Clientset) *v1beta1.DeploymentList {
	deployments, err := cs.ExtensionsV1beta1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		// Log any error but continue gracefully
		log.Println(err)
		return &v1beta1.DeploymentList{}
	}
	return deployments
}

// Just return the in-memory aggregate deployment struct as JSON
func aggregateDeploymentHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, aggregateDeployments)
}

// Just return the in-memory local deployment struct as JSON
func deploymentHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, localDeployments)
}
