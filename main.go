package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	rest2 "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var deployments map[string]*v1beta1.DeploymentList
var kubeconfig *string

type configAndClientSet struct {
	config    *rest2.Config
	clientset *kubernetes.Clientset
}

func main() {
	deployments = make(map[string]*v1beta1.DeploymentList)

	// Refresh from kube-API every X seconds
	go getDeploymentLoop(1)

	// Setup gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Serve the static frontend
	router.Use(static.Serve("/", static.LocalFile("./views", true)))

	// Define API routes
	api := router.Group("/api")
	{
		api.GET("/deployments", deploymentHandler)
	}

	// Start the listener
	fmt.Println("Listening on port 3000...")
	router.Run(":3000")
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest2.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

func getDeploymentLoop(delay time.Duration) {
	// Discover kubeconfig file
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	kubeconfigpath := *kubeconfig
	config, err := clientcmd.LoadFromFile(kubeconfigpath)
	if err != nil {
		panic(err.Error())
	}

	var clientsets []configAndClientSet

	for context := range config.Contexts {
		cfg, err := buildConfigFromFlags(context, kubeconfigpath)
		if err != nil {
			panic(err.Error())
		}
		cs, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			panic(err.Error())
		}

		clientsets = append(clientsets, configAndClientSet{config: cfg, clientset: cs})
	}

	// Loop
	for {
		time.Sleep(delay * time.Second)
		for cs := range clientsets {
			deployments[clientsets[cs].config.Host] = getDeployments("", clientsets[cs].clientset)
		}
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
func getDeployments(namespace string, cs *kubernetes.Clientset) *v1beta1.DeploymentList {
	deployments, err := cs.ExtensionsV1beta1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		fmt.Println(err.Error())
		return &v1beta1.DeploymentList{}
	}
	return deployments
}

// Just return the deployment struct as JSON
func deploymentHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, deployments)
}
