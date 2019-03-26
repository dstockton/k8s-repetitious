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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	rest2 "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Caches the aggregated deployment lists
var aggregateDeployments map[string]*v1beta1.DeploymentList

// Caches the local deployment list
var localDeployments *v1beta1.DeploymentList

// Caches the aggregated pod lists
var aggregatePods map[string]*corev1.PodList

// Caches the local pod list
var localPods *corev1.PodList

// Main entry point
func main() {
	// Initialise the empty aggregated deployment lists
	aggregateDeployments = make(map[string]*v1beta1.DeploymentList)
	aggregatePods = make(map[string]*corev1.PodList)

	// Load list of repetious endpoints
	httpPort := getEnv("HTTP_PORT", "8080")
	remotes := strings.Split(getEnv("REPETIOUS_REMOTES", "127.0.0.1:"+httpPort), ",")

	remotePollDelay, _ := time.ParseDuration(getEnv("REMOTE_POLL_DELAY", "1") + "s")
	localPollDelay, _ := time.ParseDuration(getEnv("LOCAL_POLL_DELAY", "1") + "s")

	// Refresh from remote repetitious API every X seconds
	go getRemoteResourcesLoop(remotes, remotePollDelay)

	// Refresh from local kube API every X seconds
	go getLocalResourcesLoop(localPollDelay)

	// Setup gin (HTTP server)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Serve the static frontend
	router.Use(static.Serve("/", static.LocalFile("./views", true)))

	// Define API routes
	api := router.Group("/api")
	{
		api.GET("/aggregated-resources", aggregateResourceHandler)
		api.GET("/deployments", deploymentHandler)
		api.GET("/pods", podsHandler)
	}

	// Start the listener
	log.Println("Listening on port " + httpPort + "...")
	router.Run(":" + httpPort)
	log.Println("Exiting")
}

// Helper type for returning the aggregated resources
type aggregateResources struct {
	Deployments map[string]*v1beta1.DeploymentList `json:"deployments"`
	Pods        map[string]*corev1.PodList         `json:"pods"`
}

// Helper function to get environment variables
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Helper function - get deployment names
func collectDeploymentNames(dl *v1beta1.DeploymentList) map[string]struct{} {
	vsm := make(map[string]struct{})
	if dl == nil {
		// Empty deployment list, just return empty array
		return vsm
	}
	for _, v := range dl.Items {
		// Add '-' to the end of the name to simplify the lookup later
		vsm[v.ObjectMeta.Name+"-"] = struct{}{}
	}
	return vsm
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

// Get a list of pods from a single remote URL
func getRemotePods(url string) *corev1.PodList {
	// Setup HTTP client
	httpClient := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}

	// Initialise the request parameters...
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		// Log any error but continue gracefully
		log.Println(err)
		return &corev1.PodList{}
	}

	// Let's be a good citizen
	req.Header.Set("User-Agent", "repetitious-k8s-dashboard-v1")

	// Make the HTTP request
	res, getErr := httpClient.Do(req)
	if getErr != nil {
		// Log any error but continue gracefully
		log.Println(getErr)
		return &corev1.PodList{}
	}

	// Read the entire body of the HTTP response buffer
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		// Log any error but continue gracefully
		log.Println(readErr)
		return &corev1.PodList{}
	}

	// Ready a variable for the response
	podList := corev1.PodList{}
	// Unmarshal the JSON to PodList object
	jsonErr := json.Unmarshal(body, &podList)
	if jsonErr != nil {
		// Log any error but continue gracefully
		log.Println(jsonErr)
		return &corev1.PodList{}
	}

	return &podList
}

// Continuously retrieve deployments from each remote endpoint
func getRemoteResourcesLoop(remotes []string, delay time.Duration) {
	for {
		// For each remote agent
		for _, url := range remotes {
			aggregateDeployments[url] = getRemoteDeployments("http://" + url + "/api/deployments")
			aggregatePods[url] = getRemotePods("http://" + url + "/api/pods")
		}
		time.Sleep(delay)
	}
}

// Continuously retrieve deployments from local cluster kube API
func getLocalResourcesLoop(delay time.Duration) {
	// Get current cluster kube API configuration using standard conventions
	var config *rest2.Config
	var err2 error
	config, err := rest2.InClusterConfig()
	if err != nil {
		log.Println("InClusterConfig failed: " + err.Error())
		// In-cluster config failed; let's try .kube/config file...
		configPath := os.Getenv("HOME") + "/.kube/config"
		log.Println("Attempting to load from local config: " + configPath)
		kubeconfig := &configPath
		config, err2 = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err2 != nil {
			// It's OK to panic here as it's only at start-up
			panic(err2.Error())
		}
	}

	// Setup kube API client using aquired config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		// It's OK to panic here as it's only at start-up
		panic(err.Error())
	}

	// Loop forever...
	for {
		localDeployments = getLocalDeployments("", clientset)
		localPods = getLocalPods("", clientset)

		time.Sleep(delay)
	}
}

func getLocalPods(namespace string, cs *kubernetes.Clientset) *corev1.PodList {
	pods, err := cs.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		// Log any error but continue gracefully
		log.Println(err)
		return &corev1.PodList{}
	}

	filteredList := pods.Items[:0]
	deploymentNames := collectDeploymentNames(localDeployments)

	for _, x := range pods.Items {
		var excludeFromList bool
		if x.ObjectMeta.GenerateName == "" {
			// Orphaned pod; include it in the list
			excludeFromList = false
		} else {
			splitName := strings.Split(x.ObjectMeta.GenerateName, "-")
			splitName = splitName[:len(splitName)-2]
			searchName := strings.Join(splitName, "-") + "-"
			_, excludeFromList = deploymentNames[searchName]
		}
		if !excludeFromList {
			filteredList = append(filteredList, x)
		}
	}
	pods.Items = filteredList

	return pods
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
func aggregateResourceHandler(c *gin.Context) {
	r := aggregateResources{aggregateDeployments, aggregatePods}
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, r)
}

// Just return the in-memory local deployment struct as JSON
func deploymentHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, localDeployments)
}

// Just return the in-memory local pod struct as JSON
func podsHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, localPods)
}
